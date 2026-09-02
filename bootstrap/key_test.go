package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKeyConfigWithEnv(t *testing.T) {
	configPath := t.TempDir()
	content := []byte("type: file\nscope: prod/order-service\nfile:\n  path: /etc/kratos/root.key\n")
	if err := os.WriteFile(filepath.Join(configPath, "key.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	keyConfig, err := loadKeyConfigWithEnv(configPath, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if keyConfig == nil || keyConfig.GetType() != "file" || keyConfig.GetScope() != "prod/order-service" {
		t.Fatalf("unexpected key config: %+v", keyConfig)
	}
	if keyConfig.GetFile().GetPath() != "/etc/kratos/root.key" {
		t.Fatalf("unexpected file config: %+v", keyConfig.GetFile())
	}
}

func TestLoadKeyConfigWithEnvWithoutKey(t *testing.T) {
	configPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(configPath, "server.yaml"), []byte("server:\n  http:\n    addr: 127.0.0.1:8000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	keyConfig, err := loadKeyConfigWithEnv(configPath, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if keyConfig != nil {
		t.Fatalf("key config = %+v, want nil", keyConfig)
	}
}
