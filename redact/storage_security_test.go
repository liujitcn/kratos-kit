package redact

import (
	"context"
	"strings"
	"testing"
)

// securityValueStore 只实现本组回归需要的记录查找，其他持久化能力不得被意外调用。
type securityValueStore struct {
	StorageValueStore
	value *StorageValue
}

// Find 返回与租户、策略、记录三者匹配的旁表数据。
func (s *securityValueStore) Find(_ context.Context, tenant, policy, record int64) (*StorageValue, error) {
	if s.value == nil || s.value.TenantID != tenant || s.value.StoragePolicyID != policy || s.value.RecordID != record {
		return nil, ErrStorageValueNotFound
	}
	return s.value, nil
}

// TestMaskedUpdateRequiresOriginal 验证原样提交掩码保留原文，损坏的原文不能被静默覆盖。
func TestMaskedUpdateRequiresOriginal(t *testing.T) {
	protector, err := NewStorageProtector("security-test-key")
	if err != nil {
		t.Fatal(err)
	}
	store := &securityValueStore{}
	storage := NewRedactStorage(store, nil, protector, nil)
	policy := StorageFieldPolicy{ID: 3, TenantID: 2, TableName: "users", ColumnName: "phone", Rule: FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(value any) any {
		text := value.(string)
		return text[:3] + strings.Repeat("*", len(text)-7) + text[len(text)-4:]
	}}}
	var masked string
	var prepared *StorageValue
	masked, prepared, err = storage.PrepareString(t.Context(), policy, 0, "19999911111")
	if err != nil || prepared == nil {
		t.Fatalf("创建原文失败: %v", err)
	}
	prepared.RecordID = 22
	store.value = prepared
	var changed *StorageValue
	_, changed, err = storage.PrepareString(t.Context(), policy, 22, masked)
	if err != nil || changed != nil {
		t.Fatalf("占位值覆盖了原文: %v", err)
	}
	store.value.Ciphertext[len(store.value.Ciphertext)-1] ^= 1
	_, _, err = storage.PrepareString(t.Context(), policy, 22, masked)
	if err == nil {
		t.Fatal("损坏密文未被拒绝")
	}
}

// TestUnchangedMaskStillEncryptsNewValue 验证不改变显示内容的短字段仍保存加密原文和查询摘要。
func TestUnchangedMaskStillEncryptsNewValue(t *testing.T) {
	protector, err := NewStorageProtector("security-test-key")
	if err != nil {
		t.Fatal(err)
	}
	storage := NewRedactStorage(&securityValueStore{}, nil, protector, nil)
	policy := StorageFieldPolicy{ID: 4, TenantID: 2, TableName: "tenants", ColumnName: "name", Rule: FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(value any) any { return value }}}
	var prepared *StorageValue
	_, prepared, err = storage.PrepareString(t.Context(), policy, 0, "张")
	if err != nil || prepared == nil || len(prepared.Ciphertext) == 0 || len(prepared.Digest) == 0 {
		t.Fatalf("短字段未加密: %v", err)
	}
}
