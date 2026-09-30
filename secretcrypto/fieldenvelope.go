package secretcrypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/liujitcn/go-utils/crypto"
)

const (
	// FieldEnvelopePrefix 字段信封前缀，标识值已按字段落库密钥加密。
	FieldEnvelopePrefix = "sbox:v1:"
	// FieldEnvelopeNonceSize 字段信封随机数的字节长度。
	FieldEnvelopeNonceSize = 12
)

// FieldAAD 以点分部分构建字段信封的关联数据，建议至少包含表名和列名。
func FieldAAD(parts ...string) string {
	return strings.Join(parts, ".")
}

// IsFieldEnvelope 判断值是否已是信封密文。
func IsFieldEnvelope(value string) bool {
	return strings.HasPrefix(value, FieldEnvelopePrefix)
}

// EncryptFieldEnvelope 使用 32 字节字段落库密钥加密明文并追加信封前缀。
func EncryptFieldEnvelope(key []byte, aad, plaintext string) (string, error) {
	nonce := make([]byte, FieldEnvelopeNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成字段信封随机数: %w", err)
	}
	ciphertext, err := crypto.AesGCMEncryptWithAAD([]byte(plaintext), key, nonce, []byte(aad))
	if err != nil {
		return "", fmt.Errorf("加密字段信封失败: %w", err)
	}
	payload := append(append([]byte{}, nonce...), ciphertext...)
	return FieldEnvelopePrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecryptFieldEnvelope 解析信封密文并还原明文，密钥与关联数据必须与加密时一致。
func DecryptFieldEnvelope(key []byte, aad, envelope string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(envelope, FieldEnvelopePrefix))
	if err != nil || len(payload) <= FieldEnvelopeNonceSize {
		return "", fmt.Errorf("解析字段信封密文失败")
	}
	plaintext, err := crypto.AesGCMDecryptWithAAD(payload[FieldEnvelopeNonceSize:], key, payload[:FieldEnvelopeNonceSize], []byte(aad))
	if err != nil {
		return "", fmt.Errorf("解密字段信封失败: %w", err)
	}
	return string(plaintext), nil
}
