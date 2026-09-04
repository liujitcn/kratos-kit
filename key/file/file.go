package file

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
)

const rootKeySize = 32

// Provider 从本地文件读取密钥，适用于开发环境或挂载的 Kubernetes Secret。
type Provider struct {
	path string
}

var _ internal.Provider = (*Provider)(nil)

// New 创建固定文件路径的密钥 Provider。
func New(path string) (*Provider, error) {
	if path == "" {
		return nil, errors.New("key/file: path is empty")
	}
	if err := ensureRootKeyFile(path); err != nil {
		return nil, err
	}
	return &Provider{path: path}, nil
}

// NewFromConfig 根据 Key 配置创建本地文件密钥 Provider。
func NewFromConfig(cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetFile() == nil {
		return nil, errors.New("key/file: file config is nil")
	}
	path := cfg.GetFile().GetPath()
	if path == "" {
		path = cfg.GetRootName()
	}
	return New(path)
}

// Get 读取密钥文件内容。
func (p *Provider) Get(_ context.Context, name string) (internal.Secret, error) {
	if p == nil || p.path == "" {
		return internal.Secret{}, errors.New("key/file: provider is nil")
	}
	if name != p.path {
		return internal.Secret{}, fmt.Errorf("key/file: reference %q does not match configured path %q", name, p.path)
	}

	value, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return internal.Secret{}, fmt.Errorf("key/file: secret not found: %s", p.path)
		}
		return internal.Secret{}, fmt.Errorf("key/file: read %q: %w", p.path, err)
	}
	return internal.Secret{Value: value}, nil
}

// ensureRootKeyFile 创建缺失的根密钥文件，已存在时保持原内容不变。
func ensureRootKeyFile(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("key/file: stat %q: %w", path, err)
	}

	rootKey := make([]byte, rootKeySize)
	_, err = rand.Read(rootKey)
	if err != nil {
		return fmt.Errorf("key/file: generate root key: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("key/file: create %q: %w", path, err)
	}

	var written int
	written, err = file.Write(rootKey)
	if err != nil {
		return cleanupRootKeyFile(path, file, fmt.Errorf("key/file: write %q: %w", path, err))
	}
	if written != len(rootKey) {
		return cleanupRootKeyFile(path, file, fmt.Errorf("key/file: write %q: %w", path, io.ErrShortWrite))
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("key/file: close %q: %w", path, err)
	}
	return nil
}

// cleanupRootKeyFile 清理根密钥写入失败后留下的文件。
func cleanupRootKeyFile(path string, file *os.File, cause error) error {
	closeErr := file.Close()
	removeErr := os.Remove(path)
	if closeErr != nil || (removeErr != nil && !os.IsNotExist(removeErr)) {
		return errors.Join(cause, closeErr, removeErr)
	}
	return cause
}
