package redact

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

const storageCipherPrefix = "enc:"

// StorageProtector 提供可恢复密文和精确查询摘要能力。
type StorageProtector struct {
	encryptionKey []byte
	digestKey     []byte
}

// NewStorageProtector 根据服务端密钥创建存储保护器。
func NewStorageProtector(secret string) (*StorageProtector, error) {
	if secret == "" {
		return nil, errors.New("存储保护密钥不能为空")
	}
	encryptionKey := sha256.Sum256([]byte("kratos-kit/redact/storage/encryption/" + secret))
	digestKey := sha256.Sum256([]byte("kratos-kit/redact/storage/digest/" + secret))
	return &StorageProtector{encryptionKey: encryptionKey[:], digestKey: digestKey[:]}, nil
}

// Encrypt 使用 AES-GCM 加密字段原文，并返回自描述密文。
func (p *StorageProtector) Encrypt(value, associatedData string) (string, error) {
	if p == nil || len(p.encryptionKey) == 0 {
		return "", errors.New("存储保护器未初始化")
	}
	block, err := aes.NewCipher(p.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建存储加密器失败: %w", err)
	}
	var aead cipher.AEAD
	aead, err = cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建存储 AEAD 失败: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = cryptorand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成存储随机数失败: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(value), []byte(associatedData))
	encoded := make([]byte, 0, len(nonce)+len(ciphertext))
	encoded = append(encoded, nonce...)
	encoded = append(encoded, ciphertext...)
	return storageCipherPrefix + base64.RawStdEncoding.EncodeToString(encoded), nil
}

// Decrypt 解密 AES-GCM 字段原文，并校验关联数据。
func (p *StorageProtector) Decrypt(value, associatedData string) (string, error) {
	if p == nil || len(p.encryptionKey) == 0 {
		return "", errors.New("存储保护器未初始化")
	}
	if len(value) <= len(storageCipherPrefix) || value[:len(storageCipherPrefix)] != storageCipherPrefix {
		return "", errors.New("字段未使用存储加密格式")
	}
	encoded, err := base64.RawStdEncoding.DecodeString(value[len(storageCipherPrefix):])
	if err != nil {
		return "", fmt.Errorf("解析字段密文失败: %w", err)
	}
	var block cipher.Block
	block, err = aes.NewCipher(p.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建存储解密器失败: %w", err)
	}
	var aead cipher.AEAD
	aead, err = cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建存储 AEAD 失败: %w", err)
	}
	if len(encoded) <= aead.NonceSize() {
		return "", errors.New("字段密文长度无效")
	}
	var plaintext []byte
	plaintext, err = aead.Open(nil, encoded[:aead.NonceSize()], encoded[aead.NonceSize():], []byte(associatedData))
	if err != nil {
		return "", fmt.Errorf("解密字段原文失败: %w", err)
	}
	return string(plaintext), nil
}

// Digest 使用 HMAC-SHA256 生成字段精确查询摘要。
func (p *StorageProtector) Digest(value, associatedData string) ([]byte, error) {
	if p == nil || len(p.digestKey) == 0 {
		return nil, errors.New("存储摘要保护器未初始化")
	}
	mac := hmac.New(sha256.New, p.digestKey)
	_, err := mac.Write([]byte(associatedData))
	if err != nil {
		return nil, fmt.Errorf("生成字段摘要上下文失败: %w", err)
	}
	_, err = mac.Write([]byte{0})
	if err != nil {
		return nil, fmt.Errorf("生成字段摘要分隔符失败: %w", err)
	}
	_, err = mac.Write([]byte(value))
	if err != nil {
		return nil, fmt.Errorf("生成字段查询摘要失败: %w", err)
	}
	return mac.Sum(nil), nil
}

// RuleFingerprint 根据规则类型和参数生成稳定规则指纹。
func RuleFingerprint(ruleType, ruleJSON string) string {
	digest := sha256.Sum256([]byte(ruleType + "\x00" + ruleJSON))
	return fmt.Sprintf("%x", digest[:])
}
