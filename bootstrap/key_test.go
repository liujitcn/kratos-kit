package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadKeyConfigWithEnv 验证 key 配置支持环境文件覆盖且只执行一次读取。
func TestLoadKeyConfigWithEnv(t *testing.T) {
	directory := t.TempDir()
	baseConfig := []byte("type: file\nscope: default\nroot_version: v1\nfile:\n  path: /tmp/base.key\n")
	environmentConfig := []byte("file:\n  path: /tmp/dev.key\n")
	if err := os.WriteFile(filepath.Join(directory, "key.yaml"), baseConfig, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "key.dev.yaml"), environmentConfig, 0600); err != nil {
		t.Fatal(err)
	}

	keyConfig, err := loadKeyConfigWithEnv(directory, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if keyConfig.GetType() != "file" {
		t.Fatalf("unexpected key provider type: %q", keyConfig.GetType())
	}
	if keyConfig.GetFile().GetPath() != "/tmp/dev.key" {
		t.Fatalf("unexpected environment key path: %q", keyConfig.GetFile().GetPath())
	}
}
