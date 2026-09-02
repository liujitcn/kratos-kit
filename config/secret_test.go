package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/config"
)

func TestSecretCipherEncryptDecrypt(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "redis-password"
	ciphertext, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext is not marked: %q", ciphertext)
	}
	if !strings.HasPrefix(ciphertext, "ENC[") || strings.Contains(ciphertext, "config:") {
		t.Fatalf("unexpected ciphertext format: %q", ciphertext)
	}

	decrypted, err := cipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted value = %q, want %q", decrypted, plaintext)
	}
}

func TestSecretCipherEncryptIsIdempotent(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := cipher.Encrypt("value")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := cipher.Encrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if actual != ciphertext {
		t.Fatalf("idempotent encryption changed value: got %q, want %q", actual, ciphertext)
	}
}

func TestSecretCipherDecryptPlaintext(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := cipher.Decrypt("plain-value")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "plain-value" {
		t.Fatalf("plaintext changed: got %q", actual)
	}
}

func TestSecretCipherDecoder(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt("redis-password")
	if err != nil {
		t.Fatal(err)
	}

	target := make(map[string]any)
	source := &config.KeyValue{
		Key:    "config.yaml",
		Value:  []byte("data:\n  redis:\n    password: " + ciphertext + "\n    db: 2\n"),
		Format: "yaml",
	}
	if err = cipher.Decoder()(source, target); err != nil {
		t.Fatal(err)
	}
	data, ok := target["data"].(map[string]any)
	if !ok {
		t.Fatalf("data has type %T, want map[string]any", target["data"])
	}
	redis, ok := data["redis"].(map[string]any)
	if !ok {
		t.Fatalf("redis has type %T, want map[string]any", data["redis"])
	}
	if got := redis["password"]; got != "redis-password" {
		t.Fatalf("password = %v, want redis-password", got)
	}
	if got := redis["db"]; got != 2 {
		t.Fatalf("db = %v, want 2", got)
	}
}

func TestSecretCipherDecoderPlainKeyValue(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{6}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt("password")
	if err != nil {
		t.Fatal(err)
	}

	target := make(map[string]any)
	source := &config.KeyValue{Key: "data.redis.password", Value: []byte(ciphertext)}
	if err = cipher.Decoder()(source, target); err != nil {
		t.Fatal(err)
	}
	data := target["data"].(map[string]any)
	redis := data["redis"].(map[string]any)
	if got := redis["password"]; got != "password" {
		t.Fatalf("password = %v, want password", got)
	}
}

func TestSecretCipherEncryptMarkedConfig(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("service: demo\ndata:\n  redis:\n    password: ENC(redis-password)\n    username: redis-user\n    db: 2\n  secret_size: 20\ntokens:\n  - ENC(token-value)\n")

	encrypted, err := cipher.EncryptMarkedConfig(content, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encrypted), "password: ENC[") {
		t.Fatalf("password was not encrypted: %s", encrypted)
	}
	if strings.Contains(string(encrypted), "password: redis-password") {
		t.Fatalf("password remained plaintext: %s", encrypted)
	}
	if !strings.Contains(string(encrypted), "username: redis-user") {
		t.Fatalf("unmarked field was changed: %s", encrypted)
	}
	if !strings.Contains(string(encrypted), "secret_size: 20") {
		t.Fatalf("non-secret field was changed: %s", encrypted)
	}

	target := make(map[string]any)
	source := &config.KeyValue{Key: "config.yaml", Value: encrypted, Format: "yaml"}
	if err = cipher.Decoder()(source, target); err != nil {
		t.Fatal(err)
	}
	data := target["data"].(map[string]any)
	redis := data["redis"].(map[string]any)
	if got := redis["password"]; got != "redis-password" {
		t.Fatalf("password = %v, want redis-password", got)
	}
	tokens, ok := target["tokens"].([]any)
	if !ok || len(tokens) != 1 || tokens[0] != "token-value" {
		t.Fatalf("tokens = %v, want [token-value]", target["tokens"])
	}
}

func TestSecretCipherEncryptMarkedConfigEmbeddedValue(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{11}, 32))
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("source: \"root:ENC(db-password)@tcp(127.0.0.1:3306)/test\"\n")

	var encrypted []byte
	encrypted, err = cipher.EncryptMarkedConfig(content, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encrypted), "db-password") {
		t.Fatalf("embedded plaintext remained: %s", encrypted)
	}
	if !strings.Contains(string(encrypted), "root:ENC[") || !strings.Contains(string(encrypted), "]@tcp(127.0.0.1:3306)/test") {
		t.Fatalf("embedded value was not encrypted in place: %s", encrypted)
	}

	target := make(map[string]any)
	source := &config.KeyValue{Key: "data.yaml", Value: encrypted, Format: "yaml"}
	if err = cipher.Decoder()(source, target); err != nil {
		t.Fatal(err)
	}
	if got := target["source"]; got != "root:db-password@tcp(127.0.0.1:3306)/test" {
		t.Fatalf("source = %v, want root:db-password@tcp(127.0.0.1:3306)/test", got)
	}
}

func TestSecretCipherDecryptEmbeddedValues(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{12}, 32))
	if err != nil {
		t.Fatal(err)
	}

	var usernameCiphertext string
	usernameCiphertext, err = cipher.Encrypt("db-user")
	if err != nil {
		t.Fatal(err)
	}
	var passwordCiphertext string
	passwordCiphertext, err = cipher.Encrypt("db-password")
	if err != nil {
		t.Fatal(err)
	}

	value := "mysql://" + usernameCiphertext + ":" + passwordCiphertext + "@tcp(127.0.0.1:3306)/test"
	var decrypted string
	decrypted, err = cipher.Decrypt(value)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "mysql://db-user:db-password@tcp(127.0.0.1:3306)/test" {
		t.Fatalf("decrypted value = %q, want mysql://db-user:db-password@tcp(127.0.0.1:3306)/test", decrypted)
	}
}

func TestSecretCipherEncryptMarkedConfigPreservesExistingCiphertext(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt("already-encrypted")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("password: ENC(password)\ncustom: " + ciphertext + "\n")

	encrypted, err := cipher.EncryptMarkedConfig(content, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encrypted), "password: ENC[") {
		t.Fatalf("marked field was not encrypted: %s", encrypted)
	}
	if !strings.Contains(string(encrypted), "custom: "+ciphertext) {
		t.Fatalf("existing ciphertext was changed: %s", encrypted)
	}
}

func TestSecretCipherEncryptMarkedConfigEmptyAndInvalidMarker(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{10}, 32))
	if err != nil {
		t.Fatal(err)
	}

	empty, err := cipher.EncryptMarkedConfig([]byte("password: ENC()\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), "password: \"\"") {
		t.Fatalf("empty marker was not replaced: %s", empty)
	}

	if _, err = cipher.EncryptMarkedConfig([]byte("password: ENC(missing\n"), "yaml"); err == nil {
		t.Fatal("invalid marker unexpectedly succeeded")
	}
}

func TestConfigProviderWithDecoder(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt("redis-password")
	if err != nil {
		t.Fatal(err)
	}

	configPath := t.TempDir()
	configFile := filepath.Join(configPath, "data.yaml")
	content := []byte("data:\n  redis:\n    password: " + ciphertext + "\n")
	if err = os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	provider, err := newConfigProviderWithDecoder([]config.Source{newFileConfigSource(configPath)}, nil, cipher.Decoder())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := provider.Close(); closeErr != nil {
			t.Errorf("close provider: %v", closeErr)
		}
	})
	if err = provider.Load(); err != nil {
		t.Fatal(err)
	}
	password, err := provider.Value("data.redis.password").String()
	if err != nil {
		t.Fatal(err)
	}
	if password != "redis-password" {
		t.Fatalf("password = %q, want redis-password", password)
	}
}

func TestSecretCipherRejectsWrongKeyAndTamperedValue(t *testing.T) {
	cipher, err := NewSecretCipher(bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt("value")
	if err != nil {
		t.Fatal(err)
	}

	wrongKey, err := NewSecretCipher(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = wrongKey.Decrypt(ciphertext); err == nil {
		t.Fatal("decrypt with wrong key unexpectedly succeeded")
	}

	tampered := ciphertext[:len(ciphertext)-2] + "aa]"
	if _, err = cipher.Decrypt(tampered); err == nil {
		t.Fatal("decrypt of tampered value unexpectedly succeeded")
	}
}

func TestNewSecretCipherValidation(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{name: "invalid key length", key: bytes.Repeat([]byte{1}, 16)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewSecretCipher(test.key); err == nil {
				t.Fatal("NewSecretCipher unexpectedly succeeded")
			}
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "plain", want: false},
		{value: "ENC[payload]", want: true},
		{value: "ENC[]", want: false},
		{value: "ENC[config:payload]", want: false},
	}
	for _, test := range tests {
		if got := IsEncrypted(test.value); got != test.want {
			t.Errorf("IsEncrypted(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}

func TestNewEnvironmentFileConfigSourcesSkipsKeyFiles(t *testing.T) {
	configPath := t.TempDir()
	files := map[string]string{
		"root.key":     "test-root-key",
		"key.yaml":     "type: file\n",
		"key.dev.yaml": "scope: dev\n",
		"server.yaml":  "server:\n  http:\n    addr: 127.0.0.1:8000\n",
	}

	var err error
	for name, content := range files {
		err = os.WriteFile(filepath.Join(configPath, name), []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	var sources []config.Source
	sources, err = newEnvironmentFileConfigSources(configPath, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("config sources = %d, want 1", len(sources))
	}
}
