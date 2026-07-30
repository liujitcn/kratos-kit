// Package oauth2 提供 RFC 7662 Token Introspection 认证器。
package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

var (
	// ErrMissingIntrospectionURL 表示没有配置 Token Introspection 地址。
	ErrMissingIntrospectionURL = errors.New("oauth2: introspection URL is required")
	// ErrMissingClientToken 表示没有配置客户端 Bearer token。
	ErrMissingClientToken = errors.New("oauth2: client token is required")
)

// Option 配置 OAuth2 introspection 认证器。
type Option func(*options)

type options struct {
	introspectionURL string
	clientID         string
	clientSecret     string
	clientToken      string
	httpClient       *http.Client
}

// WithIntrospectionURL 配置 RFC 7662 Token Introspection 地址。
func WithIntrospectionURL(introspectionURL string) Option {
	return func(options *options) {
		options.introspectionURL = introspectionURL
	}
}

// WithClientCredentials 配置 introspection 请求使用的客户端凭证。
func WithClientCredentials(clientID string, clientSecret string) Option {
	return func(options *options) {
		options.clientID = clientID
		options.clientSecret = clientSecret
	}
}

// WithClientToken 配置客户端请求透传的外部 Bearer token。
func WithClientToken(clientToken string) Option {
	return func(options *options) {
		options.clientToken = clientToken
	}
}

// WithHTTPClient 配置 introspection 请求使用的 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *options) {
		options.httpClient = httpClient
	}
}

// Authenticator 通过 RFC 7662 接口校验访问令牌。
type Authenticator struct {
	options    *options
	httpClient *http.Client
	ownsClient bool
}

var _ engine.Authenticator = (*Authenticator)(nil)

// NewAuthenticator 创建 OAuth2 introspection 认证器。
func NewAuthenticator(opts ...Option) (engine.Authenticator, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if options.introspectionURL == "" {
		return nil, ErrMissingIntrospectionURL
	}
	httpClient := options.httpClient
	ownsClient := false
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
		ownsClient = true
	}
	return &Authenticator{
		options:    options,
		httpClient: httpClient,
		ownsClient: ownsClient,
	}, nil
}

// Authenticate 从请求上下文读取 Bearer token 并继承请求 context 完成校验。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, _ any) (*engine.AuthClaims, error) {
	token, err := engine.AuthFromMD(ctx, engine.BearerWord, contextType)
	if err != nil {
		return nil, engine.ErrMissingBearerToken
	}
	return a.introspect(ctx, token)
}

// AuthenticateToken 使用后台 context 校验 Bearer token。
func (a *Authenticator) AuthenticateToken(token string) (*engine.AuthClaims, error) {
	return a.introspect(context.Background(), token)
}

// CreateIdentityWithContext 把显式配置的外部 Bearer token 写入客户端请求。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, _ any) (context.Context, error) {
	token, err := a.CreateIdentity(claims)
	if err != nil {
		return ctx, err
	}
	return engine.MDWithAuth(ctx, engine.BearerWord, token, contextType), nil
}

// CreateIdentity 返回显式配置的外部 Bearer token。
func (a *Authenticator) CreateIdentity(engine.AuthClaims) (string, error) {
	if a.options.clientToken == "" {
		return "", ErrMissingClientToken
	}
	return a.options.clientToken, nil
}

// Close 关闭认证器自建 HTTP 客户端的空闲连接。
func (a *Authenticator) Close() {
	if a.ownsClient {
		a.httpClient.CloseIdleConnections()
	}
}

// introspect 调用 RFC 7662 接口并转换声明。
func (a *Authenticator) introspect(ctx context.Context, token string) (*engine.AuthClaims, error) {
	form := url.Values{"token": []string{token}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.options.introspectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth2: create introspection request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if a.options.clientID != "" || a.options.clientSecret != "" {
		request.SetBasicAuth(a.options.clientID, a.options.clientSecret)
	}
	var response *http.Response
	response, err = a.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("oauth2: introspect token: %w", err)
	}
	var body []byte
	body, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("oauth2: read introspection response: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("oauth2: close introspection response: %w", closeErr)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth2: introspection returned HTTP %d: %s", response.StatusCode, string(body))
	}
	claims := make(map[string]any)
	if err = json.Unmarshal(body, &claims); err != nil {
		return nil, fmt.Errorf("oauth2: decode introspection response: %w", err)
	}
	active, ok := claims["active"].(bool)
	if !ok || !active {
		return nil, engine.ErrUnauthenticated
	}
	delete(claims, "active")
	if scope, matched := claims[engine.ClaimFieldScope].(string); matched {
		claims[engine.ClaimFieldScope] = strings.Fields(scope)
	}
	authClaims := engine.AuthClaims(claims)
	return &authClaims, nil
}
