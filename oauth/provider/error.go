package provider

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidToken 表示 OAuth token 缺失或无效。
	ErrInvalidToken = errors.New("oauth: invalid token")

	// ErrInvalidState 表示 OAuth state 校验失败。
	ErrInvalidState = errors.New("oauth: invalid state")

	// ErrMissingCode 表示 OAuth callback 缺少 code。
	ErrMissingCode = errors.New("oauth: missing code")

	// ErrUnsupportedGrantType 表示 Provider 不支持指定的 OAuth 授权类型。
	ErrUnsupportedGrantType = errors.New("oauth: unsupported grant type")
)

// NewUnsupportedGrantTypeError 创建包含具体授权类型的错误。
func NewUnsupportedGrantTypeError(grantType GrantType) error {
	return fmt.Errorf("%w: %q", ErrUnsupportedGrantType, grantType)
}

// ProviderAPIError 表示第三方 OAuth Provider 返回的业务错误。
type ProviderAPIError struct {
	Provider Type
	Code     string
	Message  string
	Raw      []byte
}

// Error 返回第三方 OAuth Provider 错误文本。
func (e *ProviderAPIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("oauth: %s api error: %s", e.Provider, e.Message)
	}
	return fmt.Sprintf("oauth: %s api error %s: %s", e.Provider, e.Code, e.Message)
}
