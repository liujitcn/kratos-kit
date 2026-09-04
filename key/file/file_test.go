package file

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/key/internal"
)

// TestNewCreatesAndReusesRootKey 验证本地 Provider 初始化时创建并复用根密钥。
func TestNewCreatesAndReusesRootKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "root.key")

	_, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat root key: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("root key mode = %o, want 600", info.Mode().Perm())
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first root key: %v", err)
	}
	if len(first) != rootKeySize {
		t.Fatalf("root key size = %d, want %d", len(first), rootKeySize)
	}

	_, err = New(path)
	if err != nil {
		t.Fatalf("New() second call error = %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second root key: %v", err)
	}
	if string(second) != string(first) {
		t.Fatal("root key changed on second initialization")
	}
}

// TestDerivedKeyIgnoresFileModificationTime 验证文件修改时间变化不会影响派生密钥。
func TestDerivedKeyIgnoresFileModificationTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "root.key")
	rootKey := bytes.Repeat([]byte{1}, rootKeySize)
	if err := os.WriteFile(path, rootKey, 0600); err != nil {
		t.Fatalf("write root key: %v", err)
	}

	provider, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resolver, err := internal.NewResolver(provider, path, "test")
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	first, err := resolver.Derive(context.Background(), "config")
	if err != nil {
		t.Fatalf("first Derive() error = %v", err)
	}
	if err := os.Chtimes(path, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatalf("change root key modification time: %v", err)
	}
	second, err := resolver.Derive(context.Background(), "config")
	if err != nil {
		t.Fatalf("second Derive() error = %v", err)
	}
	if !bytes.Equal(second, first) {
		t.Fatal("derived key changed after root key modification time changed")
	}
}
