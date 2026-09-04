package file

import (
	"os"
	"path/filepath"
	"testing"
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
