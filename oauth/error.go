package oauth

import (
	"fmt"

	"github.com/liujitcn/kratos-kit/oauth/provider"
)

var (
	// ErrInvalidToken 表示 OAuth token 缺失或无效。
	ErrInvalidToken = provider.ErrInvalidToken

	// ErrInvalidState 表示 OAuth state 校验失败。
	ErrInvalidState = provider.ErrInvalidState

	// ErrMissingCode 表示 OAuth callback 缺少 code。
	ErrMissingCode = provider.ErrMissingCode

	// ErrUnsupportedGrantType 表示 Provider 不支持指定的 OAuth 授权类型。
	ErrUnsupportedGrantType = provider.ErrUnsupportedGrantType
)

// ProviderNotFoundError 表示指定名称的 Provider 不存在。
type ProviderNotFoundError struct {
	Name Type
}

// NewProviderNotFoundError 创建 Provider 不存在错误。
func NewProviderNotFoundError(name Type) error {
	return &ProviderNotFoundError{
		Name: name,
	}
}

// Error 返回 Provider 不存在错误文本。
func (e *ProviderNotFoundError) Error() string {
	return fmt.Sprintf("oauth: provider not found: %s", e.Name)
}

// ProviderAPIError 表示第三方 OAuth Provider 返回的业务错误。
type ProviderAPIError = provider.ProviderAPIError
