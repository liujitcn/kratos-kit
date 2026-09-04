package redact

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrStorageValueNotFound 表示旁表中不存在对应的敏感值。
var ErrStorageValueNotFound = errors.New("敏感值不存在")

// StorageFieldPolicy 描述一个敏感字段的存储策略和实体映射。
type StorageFieldPolicy struct {
	FieldRef           string
	EntityRef          string
	TableName          string
	ColumnName         string
	RecordKeyField     string
	Mode               StorageMode
	SearchMode         SearchMode
	StorageRuleID      int64
	StorageRuleVersion int32
	StorageRule        FieldPolicy
}

// StorageValue 表示旁表中保存的敏感字段值。
type StorageValue struct {
	ID                 int64
	TenantID           int64
	EntityRef          string
	RecordKey          string
	FieldRef           string
	StorageMode        StorageMode
	Ciphertext         []byte
	Digest             []byte
	StorageRuleVersion int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// StoragePolicyResolver 提供响应策略和存储策略解析能力。
type StoragePolicyResolver interface {
	PolicyResolver
	ResolveStorage(context.Context, string) (StorageFieldPolicy, bool)
	ListStoragePolicies(context.Context, string) []StorageFieldPolicy
}

// StorageValueStore 提供敏感值旁表的持久化能力。
type StorageValueStore interface {
	Find(context.Context, int64, string, string, string) (*StorageValue, error)
	ListByRecord(context.Context, int64, string, string) ([]*StorageValue, error)
	ListByTenant(context.Context, []int64) ([]*StorageValue, error)
	ListByDigest(context.Context, int64, string, string, []byte) ([]*StorageValue, error)
	Save(context.Context, *StorageValue) error
	Delete(context.Context, *StorageValue) error
}

// EntityFieldAccessor 提供通用实体字段读写能力。
type EntityFieldAccessor interface {
	ValueOf(context.Context, any, string) (any, bool, error)
	Set(context.Context, any, string, any) error
}

// RedactStorage 提供通用敏感字段存储保护和旁表原文管理能力。
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

// PrepareString 根据字段存储策略生成主表值和待写入的旁表记录。
func (s *RedactStorage) PrepareString(ctx context.Context, fieldRef string, tenantID int64, value string) (string, *StorageValue, error) {
	if s == nil || s.policyResolver == nil {
		return value, nil, nil
	}
	policy, ok := s.policyResolver.ResolveStorage(ctx, fieldRef)
	if !ok || policy.Mode == StorageModePlain {
		return value, nil, nil
	}
	if value == "" {
		return "", nil, nil
	}
	if s.protector == nil {
		return "", nil, errors.New("存储保护器未初始化")
	}
	if policy.EntityRef == "" || policy.FieldRef == "" {
		return "", nil, fmt.Errorf("敏感字段 %s 缺少实体映射", fieldRef)
	}
	associatedData := storageAssociatedData(tenantID, policy.EntityRef, policy.FieldRef)
	switch policy.Mode {
	case StorageModeMask:
		ciphertext, err := s.protector.Encrypt(value, associatedData)
		if err != nil {
			return "", nil, err
		}
		var digest []byte
		if policy.SearchMode == SearchModeDigest {
			digest, err = s.protector.Digest(value, storageDigestData(policy.EntityRef, policy.FieldRef))
			if err != nil {
				return "", nil, err
			}
		}
		masked, ok := policy.StorageRule.Apply(value).(string)
		if !ok {
			return "", nil, fmt.Errorf("敏感字段 %s 存储规则未返回字符串", fieldRef)
		}
		now := time.Now()
		return masked, &StorageValue{
			TenantID:           tenantID,
			EntityRef:          policy.EntityRef,
			FieldRef:           policy.FieldRef,
			StorageMode:        policy.Mode,
			Ciphertext:         []byte(ciphertext),
			Digest:             digest,
			StorageRuleVersion: policy.StorageRuleVersion,
			CreatedAt:          now,
			UpdatedAt:          now,
		}, nil
	case StorageModeHash:
		digest, err := s.protector.Digest(value, storageDigestData(policy.EntityRef, policy.FieldRef))
		if err != nil {
			return "", nil, err
		}
		return hex.EncodeToString(digest), nil, nil
	default:
		return "", nil, fmt.Errorf("字段 %s 存储方式不支持: %d", fieldRef, policy.Mode)
	}
}

// PrepareEntity 根据实体存储策略批量处理模型中的敏感字符串字段。
func (s *RedactStorage) PrepareEntity(ctx context.Context, tenantID int64, entityRef string, entity any) (map[string]*StorageValue, error) {
	if s == nil || s.policyResolver == nil || entity == nil {
		return nil, nil
	}
	if s.fieldAccessor == nil {
		return nil, errors.New("实体字段访问器未初始化")
	}
	policies := s.policyResolver.ListStoragePolicies(ctx, entityRef)
	if len(policies) == 0 {
		return nil, nil
	}
	prepared := make(map[string]*StorageValue)
	for _, policy := range policies {
		value, zero, err := s.fieldAccessor.ValueOf(ctx, entity, policy.ColumnName)
		if err != nil {
			return nil, fmt.Errorf("读取实体 %s 字段 %s 失败: %w", entityRef, policy.ColumnName, err)
		}
		if zero || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("实体 %s 字段 %s 不是字符串", entityRef, policy.ColumnName)
		}
		stored, protected, err := s.PrepareString(ctx, policy.FieldRef, tenantID, text)
		if err != nil {
			return nil, err
		}
		if err = s.fieldAccessor.Set(ctx, entity, policy.ColumnName, stored); err != nil {
			return nil, fmt.Errorf("写入实体 %s 字段 %s 失败: %w", entityRef, policy.ColumnName, err)
		}
		if protected != nil {
			prepared[policy.FieldRef] = protected
		}
	}
	return prepared, nil
}

// SavePreparedValues 保存实体批量处理后生成的旁表敏感值。
func (s *RedactStorage) SavePreparedValues(ctx context.Context, values map[string]*StorageValue, recordKey string) error {
	var err error
	for _, value := range values {
		err = s.SavePrepared(ctx, value, recordKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteEntity 删除实体下指定业务记录的全部旁表敏感值。
func (s *RedactStorage) DeleteEntity(ctx context.Context, tenantID int64, entityRef, recordKey string) error {
	if s == nil || s.valueStore == nil {
		return nil
	}
	values, err := s.valueStore.ListByRecord(ctx, tenantID, entityRef, recordKey)
	if err != nil {
		return err
	}
	for _, value := range values {
		if err = s.valueStore.Delete(ctx, value); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTenant 删除租户下全部旁表敏感值。
func (s *RedactStorage) DeleteTenant(ctx context.Context, tenantIDs []int64) error {
	if s == nil || s.valueStore == nil || len(tenantIDs) == 0 {
		return nil
	}
	values, err := s.valueStore.ListByTenant(ctx, tenantIDs)
	if err != nil {
		return err
	}
	for _, value := range values {
		if err = s.valueStore.Delete(ctx, value); err != nil {
			return err
		}
	}
	return nil
}

// SavePrepared 保存创建或更新后已经生成主键的旁表敏感值。
func (s *RedactStorage) SavePrepared(ctx context.Context, value *StorageValue, recordKey string) error {
	if value == nil {
		return nil
	}
	if s == nil || s.valueStore == nil {
		return errors.New("敏感字段旁表存储未初始化")
	}
	if recordKey == "" {
		return errors.New("敏感字段旁表业务主键不能为空")
	}
	value.RecordKey = recordKey
	return s.valueStore.Save(ctx, value)
}

// DeletePrepared 删除指定实体字段对应的旁表敏感值。
func (s *RedactStorage) DeletePrepared(ctx context.Context, tenantID int64, entityRef, recordKey, fieldRef string) error {
	if s == nil || s.valueStore == nil {
		return errors.New("敏感字段旁表存储未初始化")
	}
	value, err := s.valueStore.Find(ctx, tenantID, entityRef, recordKey, fieldRef)
	if errors.Is(err, ErrStorageValueNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.valueStore.Delete(ctx, value)
}

// RestoreString 按存储策略恢复字段原文；无法恢复时返回主表值并标记未恢复。
func (s *RedactStorage) RestoreString(ctx context.Context, tenantID int64, entityRef, recordKey, fieldRef, stored string) (string, bool, error) {
	if s == nil || s.policyResolver == nil || s.valueStore == nil {
		return stored, false, nil
	}
	policy, ok := s.policyResolver.ResolveStorage(ctx, fieldRef)
	if !ok || policy.Mode != StorageModeMask || s.protector == nil {
		return stored, false, nil
	}
	value, err := s.valueStore.Find(ctx, tenantID, entityRef, recordKey, fieldRef)
	if errors.Is(err, ErrStorageValueNotFound) {
		return stored, false, nil
	}
	if err != nil {
		return "", false, err
	}
	plaintext, err := s.protector.Decrypt(string(value.Ciphertext), storageAssociatedData(tenantID, policy.EntityRef, policy.FieldRef))
	if err != nil {
		return "", false, err
	}
	return plaintext, true, nil
}

// PrepareResponseString 准备响应脱敏所需的字段值；存储规则与返回规则一致时直接复用主表值。
func (s *RedactStorage) PrepareResponseString(ctx context.Context, operation string, tenantID int64, entityRef, recordKey, fieldRef, stored string) (string, error) {
	if s == nil || s.policyResolver == nil || s.valueStore == nil {
		return stored, nil
	}
	responseContext := WithDirection(WithOperation(ctx, operation), DirectionResponse)
	responsePolicy, ok := s.policyResolver.Resolve(responseContext, fieldRef)
	if !ok {
		return stored, nil
	}
	storagePolicy, ok := s.policyResolver.ResolveStorage(ctx, fieldRef)
	if !ok || storagePolicy.Mode != StorageModeMask {
		return stored, nil
	}
	if responsePolicy.Mode == PolicyModeHide {
		return stored, nil
	}
	value, err := s.valueStore.Find(ctx, tenantID, entityRef, recordKey, fieldRef)
	if errors.Is(err, ErrStorageValueNotFound) {
		return stored, nil
	}
	if err != nil {
		return "", err
	}
	if responsePolicy.Mode == PolicyModeApplyRule && responsePolicy.Fingerprint == storagePolicy.StorageRule.Fingerprint && value.StorageRuleVersion == storagePolicy.StorageRuleVersion {
		return stored, nil
	}
	plaintext, err := s.protector.Decrypt(string(value.Ciphertext), storageAssociatedData(tenantID, storagePolicy.EntityRef, storagePolicy.FieldRef))
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

// PrepareResponseEntity 按接口返回策略批量准备实体敏感字段，规则一致时直接保留主表值。
func (s *RedactStorage) PrepareResponseEntity(ctx context.Context, operation string, tenantID int64, entityRef, recordKey string, entity any) error {
	if s == nil || s.policyResolver == nil || entity == nil {
		return nil
	}
	if s.fieldAccessor == nil {
		return errors.New("实体字段访问器未初始化")
	}
	policies := s.policyResolver.ListStoragePolicies(ctx, entityRef)
	if len(policies) == 0 {
		return nil
	}
	for _, policy := range policies {
		value, zero, err := s.fieldAccessor.ValueOf(ctx, entity, policy.ColumnName)
		if err != nil {
			return fmt.Errorf("读取实体 %s 字段 %s 失败: %w", entityRef, policy.ColumnName, err)
		}
		if zero || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("实体 %s 字段 %s 不是字符串", entityRef, policy.ColumnName)
		}
		prepared, err := s.PrepareResponseString(ctx, operation, tenantID, entityRef, recordKey, policy.FieldRef, text)
		if err != nil {
			return err
		}
		if err = s.fieldAccessor.Set(ctx, entity, policy.ColumnName, prepared); err != nil {
			return fmt.Errorf("写入实体 %s 字段 %s 失败: %w", entityRef, policy.ColumnName, err)
		}
	}
	return nil
}

// FindRecordKeysByDigest 按敏感字段明文查询对应的业务主键。
func (s *RedactStorage) FindRecordKeysByDigest(ctx context.Context, tenantID int64, entityRef, fieldRef, plainValue string) ([]string, error) {
	if s == nil || s.policyResolver == nil {
		return nil, nil
	}
	policy, ok := s.policyResolver.ResolveStorage(ctx, fieldRef)
	if !ok || policy.SearchMode != SearchModeDigest {
		return nil, nil
	}
	if s.valueStore == nil || s.protector == nil {
		return nil, errors.New("敏感字段查询保护器未初始化")
	}
	digest, err := s.protector.Digest(plainValue, storageDigestData(entityRef, fieldRef))
	if err != nil {
		return nil, err
	}
	values, err := s.valueStore.ListByDigest(ctx, tenantID, entityRef, fieldRef, digest)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(values))
	for _, value := range values {
		keys = append(keys, value.RecordKey)
	}
	return keys, nil
}

// HashString 使用字段级 HMAC 生成不可逆存储值。
func (s *RedactStorage) HashString(ctx context.Context, fieldRef, value string) (string, error) {
	if s == nil || s.policyResolver == nil || s.protector == nil {
		return "", errors.New("敏感字段哈希保护器未初始化")
	}
	policy, ok := s.policyResolver.ResolveStorage(ctx, fieldRef)
	if !ok || policy.Mode != StorageModeHash {
		return "", fmt.Errorf("字段 %s 未配置哈希存储", fieldRef)
	}
	digest, err := s.protector.Digest(value, storageDigestData(policy.EntityRef, policy.FieldRef))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest), nil
}

// storageAssociatedData 生成加密关联数据，防止密文跨租户或字段复用。
func storageAssociatedData(tenantID int64, entityRef, fieldRef string) string {
	return strconv.FormatInt(tenantID, 10) + "\x00" + entityRef + "\x00" + fieldRef
}

// storageDigestData 生成摘要关联数据，隔离不同实体和字段的相同原文。
func storageDigestData(entityRef, fieldRef string) string {
	return entityRef + "\x00" + fieldRef
}
