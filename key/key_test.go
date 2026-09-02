package key

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func TestNewKeyDefaultLocalProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "root.key")
	root := bytes.Repeat([]byte{7}, 32)
	if err := os.WriteFile(path, root, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &configv1.Key{
		Type:     "unknown",
		Scope:    "prod/order",
		RootName: path,
		File:     &configv1.Key_File{Path: path},
	}
	first, err := NewKey(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewKey(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	firstValue, err := first.Derive(context.Background(), "jwt")
	if err != nil {
		t.Fatal(err)
	}
	secondValue, err := second.Derive(context.Background(), "jwt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstValue, secondValue) {
		t.Fatal("same key configuration produced different derived keys")
	}
}
