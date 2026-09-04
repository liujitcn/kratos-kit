package redact

import (
	"context"
	"errors"
	"testing"
)

type storageTestResolver struct {
	responsePolicy FieldPolicy
	storagePolicy  StorageFieldPolicy
}

func (r storageTestResolver) Resolve(context.Context, string) (FieldPolicy, bool) {
	return r.responsePolicy, true
}

func (r storageTestResolver) ResolveStorage(context.Context, string) (StorageFieldPolicy, bool) {
	return r.storagePolicy, true
}

func (r storageTestResolver) ListStoragePolicies(context.Context, string) []StorageFieldPolicy {
	return []StorageFieldPolicy{r.storagePolicy}
}

type storageTestStore struct {
	values []*StorageValue
}

func (s *storageTestStore) Find(_ context.Context, tenantID int64, entityRef, recordKey, fieldRef string) (*StorageValue, error) {
	for _, value := range s.values {
		if value.TenantID == tenantID && value.EntityRef == entityRef && value.RecordKey == recordKey && value.FieldRef == fieldRef {
			return value, nil
		}
	}
	return nil, ErrStorageValueNotFound
}

func (s *storageTestStore) ListByRecord(_ context.Context, tenantID int64, entityRef, recordKey string) ([]*StorageValue, error) {
	result := make([]*StorageValue, 0)
	for _, value := range s.values {
		if value.TenantID == tenantID && value.EntityRef == entityRef && value.RecordKey == recordKey {
			result = append(result, value)
		}
	}
	return result, nil
}

func (s *storageTestStore) ListByTenant(_ context.Context, tenantIDs []int64) ([]*StorageValue, error) {
	result := make([]*StorageValue, 0)
	for _, value := range s.values {
		for _, tenantID := range tenantIDs {
			if value.TenantID == tenantID {
				result = append(result, value)
				break
			}
		}
	}
	return result, nil
}

func (s *storageTestStore) ListByDigest(_ context.Context, tenantID int64, entityRef, fieldRef string, digest []byte) ([]*StorageValue, error) {
	result := make([]*StorageValue, 0)
	for _, value := range s.values {
		if tenantID > 0 && value.TenantID != tenantID {
			continue
		}
		if value.EntityRef == entityRef && value.FieldRef == fieldRef && string(value.Digest) == string(digest) {
			result = append(result, value)
		}
	}
	return result, nil
}

func (s *storageTestStore) Save(_ context.Context, value *StorageValue) error {
	for index, existing := range s.values {
		if existing.TenantID == value.TenantID && existing.EntityRef == value.EntityRef && existing.RecordKey == value.RecordKey && existing.FieldRef == value.FieldRef {
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

func TestRedactStoragePrepareAndRestore(t *testing.T) {
	protector, err := NewStorageProtector("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	storageRule := FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(value any) any { return "***" }}
	resolver := storageTestResolver{
		responsePolicy: FieldPolicy{Mode: PolicyModeApplyRule, Fingerprint: "response"},
		storagePolicy: StorageFieldPolicy{
			FieldRef:           "user.phone",
			EntityRef:          "user",
			ColumnName:         "phone",
			Mode:               StorageModeMask,
			SearchMode:         SearchModeDigest,
			StorageRuleVersion: 1,
			StorageRule:        storageRule,
		},
	}
	store := &storageTestStore{}
	accessor := &storageTestAccessor{}
	storage := NewRedactStorage(store, resolver, protector, accessor)

	stored, prepared, err := storage.PrepareString(context.Background(), "user.phone", 7, "13812345678")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "***" || prepared == nil || len(prepared.Ciphertext) == 0 || len(prepared.Digest) == 0 {
		t.Fatalf("unexpected prepared value: stored=%q prepared=%#v", stored, prepared)
	}
	if err = storage.SavePreparedValues(context.Background(), map[string]*StorageValue{"user.phone": prepared}, "42"); err != nil {
		t.Fatal(err)
	}

	keys, err := storage.FindRecordKeysByDigest(context.Background(), 7, "user", "user.phone", "13812345678")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "42" {
		t.Fatalf("unexpected digest keys: %#v", keys)
	}

	restored, found, err := storage.RestoreString(context.Background(), 7, "user", "42", "user.phone", stored)
	if err != nil {
		t.Fatal(err)
	}
	if !found || restored != "13812345678" {
		t.Fatalf("unexpected restored value: value=%q found=%v", restored, found)
	}
}

func TestRedactStoragePrepareEntityUsesAccessor(t *testing.T) {
	protector, err := NewStorageProtector("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	resolver := storageTestResolver{storagePolicy: StorageFieldPolicy{
		FieldRef:    "user.phone",
		EntityRef:   "user",
		ColumnName:  "phone",
		Mode:        StorageModeMask,
		SearchMode:  SearchModeDigest,
		StorageRule: FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(value any) any { return "***" }},
	}}
	accessor := &storageTestAccessor{value: "13812345678"}
	storage := NewRedactStorage(&storageTestStore{}, resolver, protector, accessor)
	values, err := storage.PrepareEntity(context.Background(), 7, "user", new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || accessor.value != "***" {
		t.Fatalf("unexpected entity preparation: values=%#v value=%v", values, accessor.value)
	}
}
