package vault

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/vault/api"
)

type fakeLogical struct {
	secret *api.Secret
	query  map[string][]string
}

func (f *fakeLogical) ReadWithDataWithContext(_ context.Context, _ string, query map[string][]string) (*api.Secret, error) {
	f.query = query
	return f.secret, nil
}

func TestProviderGetKV2(t *testing.T) {
	logical := &fakeLogical{secret: &api.Secret{Data: map[string]interface{}{
		"data":     map[string]interface{}{"value": "root"},
		"metadata": map[string]interface{}{"version": 3},
	}}}
	provider := newProvider(logical)
	secret, err := provider.Get(context.Background(), "secret/data/root", "3")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secret.Value, []byte("root")) || secret.Version != "3" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	if logical.query["version"][0] != "3" {
		t.Fatalf("query = %+v", logical.query)
	}
}

func TestProviderMissingValue(t *testing.T) {
	provider := newProvider(&fakeLogical{secret: &api.Secret{Data: map[string]interface{}{"data": map[string]interface{}{}}}})
	_, err := provider.Get(context.Background(), "secret/data/root", "")
	if err == nil {
		t.Fatal("Get unexpectedly succeeded")
	}
}
