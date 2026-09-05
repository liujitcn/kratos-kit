package redact

import (
	"context"
	"errors"
	"testing"
)

type storageTestResolver struct {
	policies []StorageFieldPolicy
}

func (r storageTestResolver) ListStoragePolicies(_ context.Context, tableName string) []StorageFieldPolicy {
	result := make([]StorageFieldPolicy, 0, len(r.policies))
	for _, policy := range r.policies {
		if policy.TableName == tableName {
			result = append(result, policy)
		}
	}
	return result
}

type storageTestStore struct {
	values    []*StorageValue
	batchCall int
}

func (s *storageTestStore) Find(_ context.Context, storagePolicyID, recordID int64) (*StorageValue, error) {
	for _, value := range s.values {
		if value.StoragePolicyID == storagePolicyID && value.RecordID == recordID {
			return value, nil
		}
	}
	return nil, ErrStorageValueNotFound
}

func (s *storageTestStore) ListByRecords(_ context.Context, storagePolicyID int64, recordIDs []int64) ([]*StorageValue, error) {
	s.batchCall++
	result := make([]*StorageValue, 0)
	for _, value := range s.values {
		if value.StoragePolicyID != storagePolicyID {
			continue
		}
		for _, recordID := range recordIDs {
			if value.RecordID == recordID {
				result = append(result, value)
				break
			}
		}
	}
	return result, nil
}

func (s *storageTestStore) ListByDigest(_ context.Context, storagePolicyID int64, digest []byte) ([]*StorageValue, error) {
	result := make([]*StorageValue, 0)
	for _, value := range s.values {
		if value.StoragePolicyID == storagePolicyID && string(value.Digest) == string(digest) {
			result = append(result, value)
		}
	}
	return result, nil
}

func (s *storageTestStore) Save(_ context.Context, value *StorageValue) error {
	for index, existing := range s.values {
		if existing.StoragePolicyID == value.StoragePolicyID && existing.RecordID == value.RecordID {
			value.ID = existing.ID
			s.values[index] = value
			return nil
		}
	}
	value.ID = int64(len(s.values) + 1)
	s.values = append(s.values, value)
	return nil
}

func (s *storageTestStore) Delete(_ context.Context, value *StorageValue) error {
	for index, existing := range s.values {
		if existing.ID == value.ID {
			s.values = append(s.values[:index], s.values[index+1:]...)
			return nil
		}
	}
	return errors.New("敏感值不存在")
}

type storageTestAccessor struct {
	value any
}

func (a *storageTestAccessor) ValueOf(context.Context, any, string) (any, bool, error) {
	return a.value, a.value == nil, nil
}

func (a *storageTestAccessor) Set(_ context.Context, _ any, _ string, value any) error {
	a.value = value
	return nil
}

// TestRedactStoragePrepareAndRestore 验证入库脱敏、摘要查询和原文恢复。
func TestRedactStoragePrepareAndRestore(t *testing.T) {
	protector, err := NewStorageProtector("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	policy := StorageFieldPolicy{
		ID:         1001,
		TableName:  "base_user",
		ColumnName: "phone",
		Rule:       FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(any) any { return "***" }},
	}
	store := &storageTestStore{}
	storage := NewRedactStorage(store, storageTestResolver{policies: []StorageFieldPolicy{policy}}, protector, &storageTestAccessor{})
	var stored string
	var prepared *StorageValue
	stored, prepared, err = storage.PrepareString(context.Background(), policy, "13812345678")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "***" || prepared == nil || len(prepared.Ciphertext) == 0 || len(prepared.Digest) == 0 {
		t.Fatalf("入库脱敏结果错误: stored=%q prepared=%#v", stored, prepared)
	}
	if err = storage.SavePreparedValues(context.Background(), map[int64]*StorageValue{policy.ID: prepared}, 42); err != nil {
		t.Fatal(err)
	}
	var recordIDs []int64
	recordIDs, err = storage.FindRecordIDsByDigest(context.Background(), policy, "13812345678")
	if err != nil {
		t.Fatal(err)
	}
	if len(recordIDs) != 1 || recordIDs[0] != 42 {
		t.Fatalf("摘要查询结果错误: %#v", recordIDs)
	}
	var restored string
	var found bool
	restored, found, err = storage.RestoreString(context.Background(), policy, 42, stored)
	if err != nil {
		t.Fatal(err)
	}
	if !found || restored != "13812345678" {
		t.Fatalf("原文恢复结果错误: value=%q found=%v", restored, found)
	}
}

// TestRedactStoragePrepareEntityUsesAccessor 验证实体字段访问器参与入库脱敏。
func TestRedactStoragePrepareEntityUsesAccessor(t *testing.T) {
	protector, err := NewStorageProtector("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	policy := StorageFieldPolicy{
		ID:         1001,
		TableName:  "base_user",
		ColumnName: "phone",
		Rule:       FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(any) any { return "***" }},
	}
	accessor := &storageTestAccessor{value: "13812345678"}
	storage := NewRedactStorage(&storageTestStore{}, storageTestResolver{policies: []StorageFieldPolicy{policy}}, protector, accessor)
	var values map[int64]*StorageValue
	values, err = storage.PrepareEntity(context.Background(), "base_user", new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || accessor.value != "***" {
		t.Fatalf("实体入库脱敏结果错误: values=%#v value=%v", values, accessor.value)
	}
}

// TestRedactStorageRestoreEntities 验证查询结果按策略批量恢复原文。
func TestRedactStorageRestoreEntities(t *testing.T) {
	protector, err := NewStorageProtector("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	policy := StorageFieldPolicy{
		ID:         1001,
		TableName:  "base_user",
		ColumnName: "phone",
		Rule: FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(value any) any {
			return Mask(value.(string), 3, 4, "*")
		}},
	}
	store := &storageTestStore{}
	storage := NewRedactStorage(store, storageTestResolver{policies: []StorageFieldPolicy{policy}}, protector, storageTestMapAccessor{})
	entities := []ResponseEntity{
		{RecordID: 42, Entity: map[string]string{"phone": "138****5678"}},
		{RecordID: 43, Entity: map[string]string{"phone": "139****5678"}},
	}
	for _, entity := range entities {
		plain := "13812345678"
		if entity.RecordID == 43 {
			plain = "13912345678"
		}
		var prepared *StorageValue
		_, prepared, err = storage.PrepareString(context.Background(), policy, plain)
		if err != nil {
			t.Fatal(err)
		}
		if err = storage.SavePreparedValues(context.Background(), map[int64]*StorageValue{policy.ID: prepared}, entity.RecordID); err != nil {
			t.Fatal(err)
		}
	}
	if err = storage.RestoreEntities(context.Background(), []StorageFieldPolicy{policy}, entities); err != nil {
		t.Fatal(err)
	}
	if entities[0].Entity.(map[string]string)["phone"] != "13812345678" || entities[1].Entity.(map[string]string)["phone"] != "13912345678" {
		t.Fatalf("批量恢复结果错误: %#v", entities)
	}
	if store.batchCall != 1 {
		t.Fatalf("期望批量读取一次，实际读取 %d 次", store.batchCall)
	}
}

type storageTestMapAccessor struct{}

func (storageTestMapAccessor) ValueOf(_ context.Context, entity any, fieldName string) (any, bool, error) {
	value := entity.(map[string]string)[fieldName]
	return value, value == "", nil
}

func (storageTestMapAccessor) Set(_ context.Context, entity any, fieldName string, value any) error {
	entity.(map[string]string)[fieldName] = value.(string)
	return nil
}
