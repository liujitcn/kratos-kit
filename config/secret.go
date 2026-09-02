package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/liujitcn/go-utils/crypto"
)

const secretCipherPrefix = "ENC["

const secretMarkerPrefix = "ENC("

// secretKeyID 是配置加密使用的内部认证标识，不写入配置密文。
const secretKeyID = "kratos-kit:config"

var (
	// ErrInvalidSecretCipher 表示密钥加密器配置无效。
	ErrInvalidSecretCipher = errors.New("config: invalid secret cipher")
	// ErrInvalidSecretValue 表示配置中的密文格式无效。
	ErrInvalidSecretValue = errors.New("config: invalid secret value")
)

// SecretCipher 使用 AES-256-GCM 对配置中的敏感字段进行加解密。
type SecretCipher struct {
	key []byte
}

// NewSecretCipher 创建配置敏感字段加密器。
func NewSecretCipher(key []byte) (*SecretCipher, error) {
	if len(key) != 32 {
		return nil, ErrInvalidSecretCipher
	}
	return &SecretCipher{
		key: append([]byte(nil), key...),
	}, nil
}

// Encrypt 将明文编码为配置密文。
func (c *SecretCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" || IsEncrypted(plaintext) {
		return plaintext, nil
	}
	if c == nil {
		return "", ErrInvalidSecretCipher
	}

	nonce := make([]byte, crypto.AESGCMNonceSize)
	_, err := io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return "", fmt.Errorf("config: generate secret nonce: %w", err)
	}

	var ciphertext []byte
	ciphertext, err = crypto.AesGCMEncryptWithAAD([]byte(plaintext), c.key, nonce, []byte(secretKeyID))
	if err != nil {
		return "", fmt.Errorf("config: encrypt secret value: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...))
	return secretCipherPrefix + payload + "]", nil
}

// Decrypt 解密配置字段；普通明文会原样返回，便于递归处理配置树。
func (c *SecretCipher) Decrypt(value string) (string, error) {
	if value == "" || !strings.Contains(value, secretCipherPrefix) {
		return value, nil
	}
	if c == nil {
		return "", ErrInvalidSecretCipher
	}
	return c.decryptSecretValue(value)
}

// IsEncrypted 判断字符串是否为完整的配置密文标记。
func IsEncrypted(value string) bool {
	if !strings.HasPrefix(value, secretCipherPrefix) {
		return false
	}
	_, err := parseSecretValue(value)
	return err == nil
}

// Decoder 返回支持 ENC[...] 字段解密的 Kratos 配置解码器。
func (c *SecretCipher) Decoder() config.Decoder {
	return func(src *config.KeyValue, target map[string]any) error {
		if src == nil {
			return ErrInvalidSecretValue
		}
		if src.Format == "" {
			return c.decodePlainKeyValue(src, target)
		}

		codec := encoding.GetCodec(strings.ToLower(src.Format))
		if codec == nil {
			return fmt.Errorf("config: unsupported key format %q", src.Format)
		}
		if err := codec.Unmarshal(src.Value, &target); err != nil {
			return fmt.Errorf("config: decode %q: %w", src.Key, err)
		}
		if err := c.decryptConfigValue(target); err != nil {
			return fmt.Errorf("config: decrypt %q: %w", src.Key, err)
		}
		return nil
	}
}

// EncryptMarkedConfig 批量加密配置内容中显式标记为 ENC(...) 的片段。
// 普通明文和已是 ENC[...] 格式的值保持不变。
func (c *SecretCipher) EncryptMarkedConfig(data []byte, format string) ([]byte, error) {
	codec := encoding.GetCodec(strings.ToLower(strings.TrimPrefix(format, ".")))
	if codec == nil {
		return nil, fmt.Errorf("config: unsupported key format %q", format)
	}

	values := make(map[string]any)
	if err := codec.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("config: decode config: %w", err)
	}
	if err := c.encryptMarkedConfigValue(values); err != nil {
		return nil, err
	}

	encoded, err := codec.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("config: encode config: %w", err)
	}
	return encoded, nil
}

// decryptSecretValue 解密字符串中嵌入的一个或多个 ENC[...] 密文。
func (c *SecretCipher) decryptSecretValue(value string) (string, error) {
	var builder strings.Builder
	cursor := 0
	for {
		relativeStart := strings.Index(value[cursor:], secretCipherPrefix)
		if relativeStart < 0 {
			break
		}
		start := cursor + relativeStart
		relativeEnd := strings.IndexByte(value[start:], ']')
		if relativeEnd < 0 {
			return "", ErrInvalidSecretValue
		}
		end := start + relativeEnd + 1

		plaintext, err := c.decryptCiphertext(value[start:end])
		if err != nil {
			return "", err
		}
		builder.WriteString(value[cursor:start])
		builder.WriteString(plaintext)
		cursor = end
	}

	if cursor == 0 {
		return value, nil
	}
	builder.WriteString(value[cursor:])
	return builder.String(), nil
}

// decryptCiphertext 解密一个完整的 ENC[...] 密文标记。
func (c *SecretCipher) decryptCiphertext(value string) (string, error) {
	payload, err := parseSecretValue(value)
	if err != nil {
		return "", err
	}

	var encoded []byte
	encoded, err = base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("config: decode secret value: %w", err)
	}
	if len(encoded) < crypto.AESGCMNonceSize {
		return "", ErrInvalidSecretValue
	}

	nonce := encoded[:crypto.AESGCMNonceSize]
	ciphertext := encoded[crypto.AESGCMNonceSize:]
	var plaintext []byte
	plaintext, err = crypto.AesGCMDecryptWithAAD(ciphertext, c.key, nonce, []byte(secretKeyID))
	if err != nil {
		return "", fmt.Errorf("config: decrypt secret value: %w", err)
	}
	return string(plaintext), nil
}

// decodePlainKeyValue 解码没有格式标识的单值配置，并保持 Kratos 原有的字节值行为。
func (c *SecretCipher) decodePlainKeyValue(src *config.KeyValue, target map[string]any) error {
	keys := strings.Split(src.Key, ".")
	value := any(src.Value)
	if strings.Contains(string(src.Value), secretCipherPrefix) {
		plaintext, err := c.Decrypt(string(src.Value))
		if err != nil {
			return fmt.Errorf("config: decrypt %q: %w", src.Key, err)
		}
		value = plaintext
	}
	for i, key := range keys {
		if i == len(keys)-1 {
			target[key] = value
			return nil
		}
		sub := make(map[string]any)
		target[key] = sub
		target = sub
	}
	return nil
}

// decryptConfigValue 递归解密配置树中带 ENC[...] 标记的字符串。
func (c *SecretCipher) decryptConfigValue(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				plaintext, err := c.Decrypt(text)
				if err != nil {
					return fmt.Errorf("field %q: %w", key, err)
				}
				typed[key] = plaintext
				continue
			}
			if err := c.decryptConfigValue(child); err != nil {
				return fmt.Errorf("field %q: %w", key, err)
			}
		}
	case map[any]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				plaintext, err := c.Decrypt(text)
				if err != nil {
					return fmt.Errorf("field %v: %w", key, err)
				}
				typed[key] = plaintext
				continue
			}
			if err := c.decryptConfigValue(child); err != nil {
				return fmt.Errorf("field %v: %w", key, err)
			}
		}
	case []any:
		for index, child := range typed {
			if text, ok := child.(string); ok {
				plaintext, err := c.Decrypt(text)
				if err != nil {
					return fmt.Errorf("item %d: %w", index, err)
				}
				typed[index] = plaintext
				continue
			}
			if err := c.decryptConfigValue(child); err != nil {
				return fmt.Errorf("item %d: %w", index, err)
			}
		}
	}
	return nil
}

// encryptMarkedConfigValue 递归批量加密配置树中显式标记的值。
func (c *SecretCipher) encryptMarkedConfigValue(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				encrypted, marked, err := c.encryptMarkedValue(text)
				if err != nil {
					return fmt.Errorf("config: encrypt field %q: %w", key, err)
				}
				if marked {
					typed[key] = encrypted
					continue
				}
			}
			if err := c.encryptMarkedConfigValue(child); err != nil {
				return fmt.Errorf("config: encrypt field %q: %w", key, err)
			}
		}
	case map[any]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				encrypted, marked, err := c.encryptMarkedValue(text)
				if err != nil {
					return fmt.Errorf("config: encrypt field %v: %w", key, err)
				}
				if marked {
					typed[key] = encrypted
					continue
				}
			}
			if err := c.encryptMarkedConfigValue(child); err != nil {
				return fmt.Errorf("config: encrypt field %v: %w", key, err)
			}
		}
	case []any:
		for index, child := range typed {
			if text, ok := child.(string); ok {
				encrypted, marked, err := c.encryptMarkedValue(text)
				if err != nil {
					return fmt.Errorf("config: encrypt item %d: %w", index, err)
				}
				if marked {
					typed[index] = encrypted
					continue
				}
			}
			if err := c.encryptMarkedConfigValue(child); err != nil {
				return fmt.Errorf("config: encrypt item %d: %w", index, err)
			}
		}
	}
	return nil
}

// encryptMarkedValue 加密字符串中显式标记的片段，并返回是否命中过标记。
func (c *SecretCipher) encryptMarkedValue(value string) (string, bool, error) {
	if !strings.Contains(value, secretMarkerPrefix) {
		return value, false, nil
	}

	var builder strings.Builder
	cursor := 0
	for {
		relativeStart := strings.Index(value[cursor:], secretMarkerPrefix)
		if relativeStart < 0 {
			break
		}
		start := cursor + relativeStart
		relativeEnd := strings.IndexByte(value[start+len(secretMarkerPrefix):], ')')
		if relativeEnd < 0 {
			return "", true, ErrInvalidSecretValue
		}
		end := start + len(secretMarkerPrefix) + relativeEnd

		plaintext := value[start+len(secretMarkerPrefix) : end]
		encrypted, err := c.Encrypt(plaintext)
		if err != nil {
			return "", true, err
		}
		builder.WriteString(value[cursor:start])
		builder.WriteString(encrypted)
		cursor = end + 1
	}

	builder.WriteString(value[cursor:])
	return builder.String(), true, nil
}

// parseSecretValue 解析 ENC[payload] 格式的配置密文。
func parseSecretValue(value string) (string, error) {
	if !strings.HasPrefix(value, secretCipherPrefix) || !strings.HasSuffix(value, "]") {
		return "", ErrInvalidSecretValue
	}
	content := strings.TrimSuffix(strings.TrimPrefix(value, secretCipherPrefix), "]")
	if content == "" || strings.ContainsAny(content, ":]") {
		return "", ErrInvalidSecretValue
	}
	return content, nil
}
