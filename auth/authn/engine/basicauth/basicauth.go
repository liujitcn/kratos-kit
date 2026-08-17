package basicauth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

var (
	// ErrMissingValidator 表示没有配置任何用户名密码校验方式。
	ErrMissingValidator = errors.New("basicauth: users or validator is required")
	// ErrMissingClientCredentials 表示找不到客户端身份对应的密码。
	ErrMissingClientCredentials = errors.New("basicauth: client credentials are required")
)

// CredentialValidator 校验用户名和密码。
type CredentialValidator func(username string, password string) bool

// Option 配置 Basic 认证器。
type Option func(*options)

type options struct {
	users     map[string]string
	validator CredentialValidator
}

// WithUser 添加一个静态用户名密码。
func WithUser(username string, password string) Option {
	return func(options *options) {
		if options.users == nil {
			options.users = make(map[string]string)
		}
		options.users[username] = password
	}
}

// WithUsers 配置静态用户名密码集合。
func WithUsers(users map[string]string) Option {
	return func(options *options) {
		options.users = make(map[string]string, len(users))
		for username, password := range users {
			options.users[username] = password
		}
	}
}

// WithValidator 配置外部用户名密码校验函数。
func WithValidator(validator CredentialValidator) Option {
	return func(options *options) {
		options.validator = validator
	}
}

// Authenticator 校验 Basic 凭证。
type Authenticator struct {
	options *options
}

var _ engine.Authenticator = (*Authenticator)(nil)

// NewAuthenticator 创建 Basic 认证器。
func NewAuthenticator(opts ...Option) (engine.Authenticator, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if len(options.users) == 0 && options.validator == nil {
		return nil, ErrMissingValidator
	}
	return &Authenticator{options: options}, nil
}

// Authenticate 从请求上下文读取 Basic 凭证并校验。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, _ any) (*engine.AuthClaims, error) {
	token, err := engine.AuthFromMD(ctx, engine.BasicWord, contextType)
	if err != nil {
		return nil, engine.ErrMissingBearerToken
	}
	return a.AuthenticateToken(token)
}

// AuthenticateToken 解码并校验 Basic 凭证。
func (a *Authenticator) AuthenticateToken(token string) (*engine.AuthClaims, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, engine.ErrInvalidToken
	}
	username, password, found := strings.Cut(string(decoded), ":")
	if !found || !a.validate(username, password) {
		return nil, engine.ErrUnauthenticated
	}
	claims := engine.AuthClaims{engine.ClaimFieldSubject: username}
	return &claims, nil
}

// CreateIdentityWithContext 把 Basic 凭证写入客户端请求上下文。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, _ any) (context.Context, error) {
	token, err := a.CreateIdentity(claims)
	if err != nil {
		return ctx, err
	}
	return engine.MDWithAuth(ctx, engine.BasicWord, token, contextType), nil
}

// CreateIdentity 根据 subject 和静态密码创建 Basic 凭证。
func (a *Authenticator) CreateIdentity(claims engine.AuthClaims) (string, error) {
	username, err := claims.GetSubject()
	if err != nil {
		return "", err
	}
	password, ok := a.options.users[username]
	if !ok {
		return "", ErrMissingClientCredentials
	}
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password)), nil
}

// Close 释放认证器资源。
func (a *Authenticator) Close() {}

// validate 校验用户名密码。
func (a *Authenticator) validate(username string, password string) bool {
	if a.options.validator != nil {
		return a.options.validator(username, password)
	}
	expected, ok := a.options.users[username]
	return ok && subtle.ConstantTimeCompare([]byte(expected), []byte(password)) == 1
}
