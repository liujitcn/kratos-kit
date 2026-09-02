package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProviderGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "root.key")
	if err := os.WriteFile(path, []byte("root"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := provider.Get(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(secret.Value) != "root" || secret.Version == "" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	pinned, err := provider.Get(context.Background(), path, "1")
	if err != nil {
		t.Fatal(err)
	}
	if pinned.Version != "1" {
		t.Fatalf("pinned version = %q", pinned.Version)
	}
}
