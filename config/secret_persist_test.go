package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type testKey struct{}

// Derive 返回测试用的固定配置密钥。
func (testKey) Derive(context.Context, string) ([]byte, error) {
	return bytes.Repeat([]byte{1}, 32), nil
}

func TestEncryptMarkedConfig(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}

	input := []byte("# keep this comment\nsource: root:ENC(112233)@tcp(127.0.0.1:3306)/kratos_admin\npassword: ENC(112233)\n")
	output, err := encryptMarkedConfig(input, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("ENC(112233)")) {
		t.Fatalf("output still contains plaintext marker: %s", output)
	}
	if !bytes.Contains(output, []byte("ENC[")) {
		t.Fatalf("output does not contain encrypted marker: %s", output)
	}
	if !bytes.Contains(output, []byte("# keep this comment")) {
		t.Fatalf("output did not preserve comment: %s", output)
	}

	decrypted, err := cipher.Decrypt(string(output))
	if err != nil {
		t.Fatal(err)
	}
	want := "# keep this comment\nsource: root:112233@tcp(127.0.0.1:3306)/kratos_admin\npassword: 112233\n"
	if decrypted != want {
		t.Fatalf("decrypted output mismatch:\nwant: %s\n got: %s", want, decrypted)
	}
}

func TestEncryptMarkedConfigRejectsUnclosedMarker(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}

	_, err = encryptMarkedConfig([]byte("password: ENC(112233\n"), cipher)
	if err == nil {
		t.Fatal("expected unclosed marker error")
	}
}

func TestEncryptMarkedConfigFilesUsesCurrentEnvironment(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"base.yaml":      "value: ENC(base-secret)\n",
		"base.dev.yaml":  "value: ENC(dev-secret)\n",
		"base.prod.yaml": "value: ENC(prod-secret)\n",
		"plain.yaml":     "value: plain\n",
	}
	for name, content := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cipher, err := NewSecretCipher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = encryptMarkedConfigFiles(directory, "dev", cipher); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"base.yaml", "base.dev.yaml"} {
		path := filepath.Join(directory, name)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if bytes.Contains(content, []byte("ENC(")) {
			t.Fatalf("%s still contains a plaintext marker: %s", name, content)
		}
	}
	prodContent, err := os.ReadFile(filepath.Join(directory, "base.prod.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(prodContent, []byte("ENC(prod-secret)")) {
		t.Fatalf("prod config was unexpectedly rewritten: %s", prodContent)
	}

	baseContent, err := os.ReadFile(filepath.Join(directory, "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err = encryptMarkedConfigFiles(directory, "dev", cipher); err != nil {
		t.Fatal(err)
	}
	baseContentAgain, err := os.ReadFile(filepath.Join(directory, "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseContent, baseContentAgain) {
		t.Fatal("second encryption pass changed the config")
	}
}

func TestLoadBootstrapConfigEncryptsMarkedFilesBeforeLoading(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "data.yaml")
	input := []byte("data:\n  database:\n    driver: mysql\n    source: root:ENC(112233)@tcp(127.0.0.1:3306)/kratos_admin\n")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}

	cleanup, err := LoadBootstrapConfig(directory, "dev", testKey{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("ENC(112233)")) {
		t.Fatalf("bootstrap left plaintext marker in config: %s", output)
	}
	if !bytes.Contains(output, []byte("ENC[")) {
		t.Fatalf("bootstrap did not persist encrypted marker: %s", output)
	}
}
