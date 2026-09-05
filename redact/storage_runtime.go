package redact

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

// ErrStorageValueNotFound 表示旁表中不存在对应的敏感值。
var ErrStorageValueNotFound = errors.New("敏感值不存在")

// StorageFieldPolicy 描述一个数据库字段的入库脱敏策略。
type StorageFieldPolicy struct {
	ID         int64
	TableName  string
	ColumnName string
	Rule       FieldPolicy
}

// StorageValue 表示旁表中保存的敏感字段加密原文和查询摘要。
type StorageValue struct {
	ID              int64
	StoragePolicyID int64
	RecordID        int64
	Ciphertext      []byte
	Digest          []byte
}

// StoragePolicyResolver 提供数据库字段入库策略解析能力。
type StoragePolicyResolver interface {
	ListStoragePolicies(context.Context, string) []StorageFieldPolicy
}

// StorageValueStore 提供敏感值旁表的持久化能力。
type StorageValueStore interface {
	Find(context.Context, int64, int64) (*StorageValue, error)
	ListByRecords(context.Context, int64, []int64) ([]*StorageValue, error)
	ListByDigest(context.Context, int64, []byte) ([]*StorageValue, error)
	Save(context.Context, *StorageValue) error
	Delete(context.Context, *StorageValue) error
}

// ResponseEntity 描述一个需要恢复数据库敏感原文的业务实体。
type ResponseEntity struct {
	RecordID int64
	Entity   any
}

// EntityFieldAccessor 提供通用实体字段读写能力。
type EntityFieldAccessor interface {
	ValueOf(context.Context, any, string) (any, bool, error)
	Set(context.Context, any, string, any) error
}

// RedactStorage 提供通用敏感字段入库保护和查询原文恢复能力。
type RedactStorage struct {
	valueStore     StorageValueStore
	policyResolver StoragePolicyResolver
	protector      *StorageProtector
	fieldAccessor  EntityFieldAccessor
}

// NewRedactStorage 创建通用敏感字段存储保护实例。
func NewRedactStorage(
	valueStore StorageValueStore,
	policyResolver StoragePolicyResolver,
	protector *StorageProtector,
	fieldAccessor EntityFieldAccessor,
) *RedactStorage {
	return &RedactStorage{valueStore: valueStore, policyResolver: policyResolver, protector: protector, fieldAccessor: fieldAccessor}
}

// PrepareString 根据入库策略生成主表脱敏值和待写入的旁表记录。
func (s *RedactStorage) PrepareString(ctx context.Context, policy StorageFieldPolicy, value string) (string, *StorageValue, error) {
	if s == nil || value == "" {
		return value, nil, nil
	}
	if policy.ID <= 0 || policy.TableName == "" || policy.ColumnName == "" {
		return "", nil, errors.New("敏感字段入库策略无效")
	}
	if policy.Rule.Mode != PolicyModeApplyRule || policy.Rule.Transform == nil {
		return "", nil, fmt.Errorf("敏感字段 %s.%s 入库规则未初始化", policy.TableName, policy.ColumnName)
	}
	if s.protector == nil {
		return "", nil, errors.New("存储保护器未初始化")
	}
	masked, ok := policy.Rule.Apply(value).(string)
	if !ok {
		return "", nil, fmt.Errorf("敏感字段 %s.%s 入库规则未返回字符串", policy.TableName, policy.ColumnName)
	}
	// 更新接口可能把当前掩码原样提交回来，此时保留已有旁表原文。
	if masked == value {
		return value, nil, nil
	}
	ciphertext, err := s.protector.Encrypt(value, storageAssociatedData(policy.ID))
	if err != nil {
		return "", nil, err
	}
	var digest []byte
	digest, err = s.protector.Digest(value, storageDigestData(policy.ID))
	if err != nil {
		return "", nil, err
	}
	return masked, &StorageValue{StoragePolicyID: policy.ID, Ciphertext: []byte(ciphertext), Digest: digest}, nil
}

// PrepareEntity 根据物理表入库策略批量处理模型中的敏感字符串字段。
func (s *RedactStorage) PrepareEntity(ctx context.Context, tableName string, entity any) (map[int64]*StorageValue, error) {
	if s == nil || s.policyResolver == nil || entity == nil {
		return nil, nil
	}
	policies := s.policyResolver.ListStoragePolicies(ctx, tableName)
	return s.PrepareEntityWithPolicies(ctx, entity, policies)
}

// PrepareEntityWithPolicies 按指定入库策略批量处理模型中的敏感字符串字段。
func (s *RedactStorage) PrepareEntityWithPolicies(ctx context.Context, entity any, policies []StorageFieldPolicy) (map[int64]*StorageValue, error) {
	if s == nil || entity == nil || len(policies) == 0 {
		return nil, nil
	}
	if s.fieldAccessor == nil {
		return nil, errors.New("实体字段访问器未初始化")
	}
	prepared := make(map[int64]*StorageValue)
	var err error
	for _, policy := range policies {
		var value any
		var zero bool
		value, zero, err = s.fieldAccessor.ValueOf(ctx, entity, policy.ColumnName)
		if err != nil {
			return nil, fmt.Errorf("读取实体 %s 字段 %s 失败: %w", policy.TableName, policy.ColumnName, err)
		}
		if zero || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("实体 %s 字段 %s 不是字符串", policy.TableName, policy.ColumnName)
		}
		var stored string
		var protected *StorageValue
		stored, protected, err = s.PrepareString(ctx, policy, text)
		if err != nil {
			return nil, err
		}
		if err = s.fieldAccessor.Set(ctx, entity, policy.ColumnName, stored); err != nil {
			return nil, fmt.Errorf("写入实体 %s 字段 %s 失败: %w", policy.TableName, policy.ColumnName, err)
		}
		if protected != nil {
			prepared[policy.ID] = protected
		}
	}
	return prepared, nil
}

// SavePreparedValues 保存实体处理后生成的全部旁表敏感值。
func (s *RedactStorage) SavePreparedValues(ctx context.Context, values map[int64]*StorageValue, recordID int64) error {
	var err error
	for _, value := range values {
		err = s.SavePrepared(ctx, value, recordID)
		if err != nil {
			return err
		}
	}
	return nil
}

// SavePrepared 保存创建或更新后已经生成主键的旁表敏感值。
func (s *RedactStorage) SavePrepared(ctx context.Context, value *StorageValue, recordID int64) error {
	if value == nil {
		return nil
	}
	if s == nil || s.valueStore == nil {
		return errors.New("敏感字段旁表存储未初始化")
	}
	if recordID <= 0 {
		return errors.New("敏感字段旁表记录ID不能为空")
	}
	value.RecordID = recordID
	return s.valueStore.Save(ctx, value)
}

// DeletePrepared 删除指定入库策略和业务记录对应的旁表敏感值。
func (s *RedactStorage) DeletePrepared(ctx context.Context, storagePolicyID, recordID int64) error {
	if s == nil || s.valueStore == nil {
		return errors.New("敏感字段旁表存储未初始化")
	}
	value, err := s.valueStore.Find(ctx, storagePolicyID, recordID)
	if errors.Is(err, ErrStorageValueNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.valueStore.Delete(ctx, value)
}

// RestoreString 根据入库策略恢复字段原文；无法恢复时返回主表值并标记未恢复。
func (s *RedactStorage) RestoreString(ctx context.Context, policy StorageFieldPolicy, recordID int64, stored string) (string, bool, error) {
	if s == nil || s.valueStore == nil || s.protector == nil || recordID <= 0 {
		return stored, false, nil
	}
	value, err := s.valueStore.Find(ctx, policy.ID, recordID)
	if errors.Is(err, ErrStorageValueNotFound) {
		return stored, false, nil
	}
	if err != nil {
		return "", false, err
	}
	var plaintext string
	plaintext, err = s.protector.Decrypt(string(value.Ciphertext), storageAssociatedData(policy.ID))
	if err != nil {
		return "", false, err
	}
	return plaintext, true, nil
}

// RestoreEntities 按入库策略批量恢复实体中的敏感字段原文。
func (s *RedactStorage) RestoreEntities(ctx context.Context, policies []StorageFieldPolicy, entities []ResponseEntity) error {
	if s == nil || len(policies) == 0 || len(entities) == 0 {
		return nil
	}
	if s.valueStore == nil || s.protector == nil {
		return errors.New("敏感字段查询保护器未初始化")
	}
	if s.fieldAccessor == nil {
		return errors.New("实体字段访问器未初始化")
	}
	var err error
	for _, policy := range policies {
		recordIDs := make([]int64, 0, len(entities))
		for _, entity := range entities {
			if entity.Entity != nil && entity.RecordID > 0 {
				recordIDs = append(recordIDs, entity.RecordID)
			}
		}
		var values []*StorageValue
		values, err = s.valueStore.ListByRecords(ctx, policy.ID, recordIDs)
		if err != nil {
			return err
		}
		valueByRecordID := make(map[int64]*StorageValue, len(values))
		for _, value := range values {
			valueByRecordID[value.RecordID] = value
		}
		for _, entity := range entities {
			value, ok := valueByRecordID[entity.RecordID]
			if !ok {
				continue
			}
			var plaintext string
			plaintext, err = s.protector.Decrypt(string(value.Ciphertext), storageAssociatedData(policy.ID))
			if err != nil {
				return err
			}
			if err = s.fieldAccessor.Set(ctx, entity.Entity, policy.ColumnName, plaintext); err != nil {
				return fmt.Errorf("写入实体 %s 字段 %s 失败: %w", policy.TableName, policy.ColumnName, err)
			}
		}
	}
	return nil
}

// FindRecordIDsByDigest 按敏感字段明文查询对应的业务记录ID。
func (s *RedactStorage) FindRecordIDsByDigest(ctx context.Context, policy StorageFieldPolicy, plainValue string) ([]int64, error) {
	if s == nil || s.valueStore == nil || s.protector == nil {
		return nil, errors.New("敏感字段查询保护器未初始化")
	}
	digest, err := s.protector.Digest(plainValue, storageDigestData(policy.ID))
	if err != nil {
		return nil, err
	}
	var values []*StorageValue
	values, err = s.valueStore.ListByDigest(ctx, policy.ID, digest)
	if err != nil {
		return nil, err
	}
	recordIDs := make([]int64, 0, len(values))
	for _, value := range values {
		recordIDs = append(recordIDs, value.RecordID)
	}
	return recordIDs, nil
}

// storageAssociatedData 生成字段级加密关联数据。
func storageAssociatedData(storagePolicyID int64) string {
	return "storage-policy\x00" + strconv.FormatInt(storagePolicyID, 10)
}

// storageDigestData 生成字段级查询摘要关联数据。
func storageDigestData(storagePolicyID int64) string {
	return "storage-policy-digest\x00" + strconv.FormatInt(storagePolicyID, 10)
}
