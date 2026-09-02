package internal

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const keySize = 32

var (
	// ErrInvalidSecret 表示读取到的根密钥格式无效。
	ErrInvalidSecret = errors.New("key: invalid root secret")
	// ErrSecretNotFound 表示根密钥不存在或没有值。
	ErrSecretNotFound = errors.New("key: secret not found")
)

// Secret 表示 Provider 读取到的根密钥和版本。
type Secret struct {
	Value   []byte
	Version string
}

// Provider 定义内部根密钥读取能力。
type Provider interface {
	Get(context.Context, string, string) (Secret, error)
}

// Resolver 实现统一的业务密钥派生。
type Resolver struct {
	provider    Provider
	rootName    string
	rootVersion string
	scope       string
}

// NewResolver 创建内部密钥解析器。
func NewResolver(provider Provider, rootName, rootVersion, scope string) (*Resolver, error) {
	if provider == nil || rootName == "" || scope == "" {
		return nil, errors.New("key: invalid resolver config")
	}
	return &Resolver{provider: provider, rootName: rootName, rootVersion: rootVersion, scope: scope}, nil
}

// Derive 读取根密钥并按用途派生业务密钥。
func (r *Resolver) Derive(ctx context.Context, purpose string) ([]byte, error) {
	if r == nil || r.provider == nil || purpose == "" {
		return nil, errors.New("key: invalid resolver")
	}
	root, err := r.provider.Get(ctx, r.rootName, r.rootVersion)
	if err != nil {
		return nil, fmt.Errorf("key: get root key %q: %w", r.rootName, err)
	}
	rootKey, err := decodeRootKey(root.Value)
	if err != nil {
		return nil, err
	}
	version := root.Version
	if version == "" {
		version = r.rootVersion
	}
	if version == "" {
		version = "unversioned"
	}
	info := "kratos-kit/key/v1/" + encodeLabel(r.scope) + encodeLabel(purpose) + encodeLabel(version)
	derived, err := hkdf.Key(sha256.New, rootKey, nil, info, keySize)
	if err != nil {
		return nil, fmt.Errorf("key: derive key: %w", err)
	}
	return derived, nil
}

func decodeRootKey(value []byte) ([]byte, error) {
	if len(value) == keySize {
		return append([]byte(nil), value...), nil
	}
	encoded := strings.TrimSpace(string(value))
	for _, decoder := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		decoded, err := decoder.DecodeString(encoded)
		if err == nil && len(decoded) == keySize {
			return decoded, nil
		}
	}
	return nil, ErrInvalidSecret
}

func encodeLabel(value string) string {
	return strconv.Itoa(len(value)) + ":" + value + ";"
}
