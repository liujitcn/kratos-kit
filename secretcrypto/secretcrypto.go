package secretcrypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/liujitcn/go-utils/crypto"
	"github.com/liujitcn/kratos-kit/cache"
)

const (
	// ReasonInvalid 敏感字段密文校验失败的错误原因。
	ReasonInvalid = "SECRET_CRYPTO_INVALID"
	// ReasonKeyExpired 敏感字段临时密钥已过期的错误原因。
	ReasonKeyExpired = "SECRET_CRYPTO_KEY_EXPIRED"

	// Algorithm 敏感字段混合加密的算法标识。
	Algorithm = "RSA-OAEP-256+A256GCM"
	// DefaultTTL 临时私钥记录的默认有效期。
	DefaultTTL = 5 * time.Minute
	// DefaultKeyBits 临时 RSA 密钥的默认位数。
	DefaultKeyBits = 2048
	// DefaultMessageTypeName 拦截器识别的敏感密文消息全名。
	DefaultMessageTypeName = "common.v1.SecretCrypto"
	// DefaultTextFieldName 承载密文与解密明文的字段名。
	DefaultTextFieldName = "text"

	// secretCryptoKeyPrefix 临时私钥记录的缓存键前缀。
	secretCryptoKeyPrefix = "shared:auth:secret:crypto:"
	// nonceSize 临时私钥记录随机值的字节长度。
	nonceSize = 16
)

// PublicKeyInfo 下发给前端的临时公钥信息。
type PublicKeyInfo struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
	Algorithm string `json:"algorithm"`
	Nonce     string `json:"nonce"`
	ExpiresIn int64  `json:"expires_in"`
}

// Cipher 单个敏感字段密文的载体，Text 传输时承载密文，解密后回填明文。
type Cipher struct {
	KeyID        string
	Nonce        string
	Algorithm    string
	EncryptedKey string
	IV           string
	Text         string
}

// keyRecord 缓存中的临时私钥记录。
type keyRecord struct {
	PrivateKey string `json:"private_key"`
	Nonce      string `json:"nonce"`
	Algorithm  string `json:"algorithm"`
}

// Option 服务可选项。
type Option func(*Service)

// WithTTL 覆盖临时私钥记录的有效期。
func WithTTL(ttl time.Duration) Option {
	return func(s *Service) { s.ttl = ttl }
}

// WithKeyBits 覆盖临时 RSA 密钥的位数。
func WithKeyBits(bits int) Option {
	return func(s *Service) { s.keyBits = bits }
}

// WithMessageTypeName 覆盖拦截器识别的敏感密文消息全名。
func WithMessageTypeName(name string) Option {
	return func(s *Service) { s.messageType = name }
}

// WithTextFieldName 覆盖承载密文与明文的字段名。
func WithTextFieldName(name string) Option {
	return func(s *Service) { s.textField = name }
}

// Service 敏感字段临时密钥服务，负责签发一次性公钥并批量解密提交的密文。
type Service struct {
	cache       cache.Cache
	ttl         time.Duration
	keyBits     int
	messageType string
	textField   string
}

// NewService 创建敏感字段临时密钥服务。
func NewService(cacheClient cache.Cache, opts ...Option) *Service {
	s := &Service{
		cache:       cacheClient,
		ttl:         DefaultTTL,
		keyBits:     DefaultKeyBits,
		messageType: DefaultMessageTypeName,
		textField:   DefaultTextFieldName,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GeneratePublicKey 签发一次性临时公钥，私钥记录写入缓存等待解密消费。
func (s *Service) GeneratePublicKey() (*PublicKeyInfo, error) {
	rsaCrypto, err := crypto.NewRSACrypto(s.keyBits)
	if err != nil {
		return nil, fmt.Errorf("生成敏感字段临时密钥: %w", err)
	}
	privateKeyPEM, err := rsaCrypto.ExportPrivateKeyPKCS8()
	if err != nil {
		return nil, fmt.Errorf("导出敏感字段临时私钥: %w", err)
	}
	publicKeyPEM, err := rsaCrypto.ExportPublicKeyPKIX()
	if err != nil {
		return nil, fmt.Errorf("导出敏感字段临时公钥: %w", err)
	}
	nonceBytes, err := crypto.GenerateAESKey(nonceSize)
	if err != nil {
		return nil, fmt.Errorf("生成敏感字段随机值: %w", err)
	}
	keyID, err := randomKeyID()
	if err != nil {
		return nil, fmt.Errorf("生成敏感字段密钥编号: %w", err)
	}
	nonce := base64.StdEncoding.EncodeToString(nonceBytes)
	recordBytes, err := json.Marshal(keyRecord{PrivateKey: privateKeyPEM, Nonce: nonce, Algorithm: Algorithm})
	if err != nil {
		return nil, fmt.Errorf("序列化敏感字段私钥记录: %w", err)
	}
	if err = s.cache.Set(secretCryptoKeyPrefix+keyID, string(recordBytes), s.ttl); err != nil {
		return nil, fmt.Errorf("保存敏感字段私钥记录: %w", err)
	}
	return &PublicKeyInfo{
		KeyID:     keyID,
		PublicKey: publicKeyPEM,
		Algorithm: Algorithm,
		Nonce:     nonce,
		ExpiresIn: int64(s.ttl / time.Second),
	}, nil
}

// Decrypt 批量解密同一请求提交的敏感字段密文并回填 Plaintext。
// 同一 KeyID 的多个密文只消费一次私钥记录，取出即删。
func (s *Service) Decrypt(ciphers ...*Cipher) error {
	groups := make(map[string][]*Cipher, len(ciphers))
	for _, item := range ciphers {
		if item == nil {
			continue
		}
		groups[item.KeyID] = append(groups[item.KeyID], item)
	}
	for keyID, group := range groups {
		recordText, err := s.cache.GetDel(secretCryptoKeyPrefix + keyID)
		if err != nil || recordText == "" {
			return errKeyExpired()
		}
		var record keyRecord
		if err = json.Unmarshal([]byte(recordText), &record); err != nil {
			return errInvalid("敏感字段密钥记录无效")
		}
		rsaCrypto, err := crypto.NewRSACryptoFromPrivateKeyPEM(record.PrivateKey)
		if err != nil {
			return errInvalid("敏感字段密钥记录无效")
		}
		for _, item := range group {
			if err = decryptCipher(rsaCrypto, record, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// decryptCipher 使用已取出的私钥记录解密单个密文并回填明文。
func decryptCipher(rsaCrypto *crypto.RSACrypto, record keyRecord, item *Cipher) error {
	if item.KeyID == "" || item.Nonce == "" || item.Algorithm == "" ||
		item.EncryptedKey == "" || item.IV == "" || item.Text == "" {
		return errInvalid("敏感字段密文不完整")
	}
	if item.Algorithm != Algorithm || record.Algorithm != Algorithm {
		return errInvalid("敏感字段加密算法不支持")
	}
	if record.Nonce != item.Nonce {
		return errInvalid("敏感字段随机值无效")
	}
	aesKey, err := rsaCrypto.DecryptBytes(item.EncryptedKey)
	if err != nil {
		return errInvalid("敏感字段密钥解密失败")
	}
	iv, err := base64.StdEncoding.DecodeString(item.IV)
	if err != nil {
		return errInvalid("敏感字段初始化向量无效")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(item.Text)
	if err != nil {
		return errInvalid("敏感字段密文无效")
	}
	plaintext, err := crypto.AesGCMDecrypt(ciphertext, aesKey, iv)
	if err != nil {
		return errInvalid("敏感字段解密失败")
	}
	item.Text = string(plaintext)
	return nil
}

// randomKeyID 生成随机密钥编号。
func randomKeyID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// errInvalid 构建敏感字段密文无效错误。
func errInvalid(message string) error {
	return errors.New(400, ReasonInvalid, message)
}

// errKeyExpired 构建敏感字段临时密钥过期错误。
func errKeyExpired() error {
	return errors.New(400, ReasonKeyExpired, "敏感字段密钥已过期，请重新提交")
}
