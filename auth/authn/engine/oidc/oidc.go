package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	keyfuncV3 "github.com/MicahParks/keyfunc/v3"
	jwtV5 "github.com/golang-jwt/jwt/v5"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

var (
	// ErrMissingIssuer 表示没有配置 OIDC issuer。
	ErrMissingIssuer = errors.New("oidc: issuer is required")
	// ErrMissingAudience 表示没有配置 OIDC audience。
	ErrMissingAudience = errors.New("oidc: audience is required")
	// ErrMissingClientToken 表示没有配置客户端使用的 OIDC token。
	ErrMissingClientToken = errors.New("oidc: client token is required")
)

var defaultSigningMethods = []string{
	"RS256", "RS384", "RS512",
	"ES256", "ES384", "ES512",
	"PS256", "PS384", "PS512",
	"EdDSA",
}

type providerConfig struct {
	Issuer        string   `json:"issuer"`
	JWKSURL       string   `json:"jwks_uri"`
	SigningMethod []string `json:"id_token_signing_alg_values_supported"`
}

// Option 配置 OIDC 认证器。
type Option func(*options)

type options struct {
	ctx            context.Context
	issuer         string
	audience       string
	clientToken    string
	signingMethods []string
	httpClient     *http.Client
}

// WithContext 配置 discovery 和 JWKS 刷新的父 context。
func WithContext(ctx context.Context) Option {
	return func(options *options) {
		options.ctx = ctx
	}
}

// WithIssuer 配置 OIDC issuer。
func WithIssuer(issuer string) Option {
	return func(options *options) {
		options.issuer = issuer
	}
}

// WithAudience 配置 OIDC audience。
func WithAudience(audience string) Option {
	return func(options *options) {
		options.audience = audience
	}
}

// WithClientToken 配置客户端请求透传的外部 OIDC token。
func WithClientToken(clientToken string) Option {
	return func(options *options) {
		options.clientToken = clientToken
	}
}

// WithSigningMethods 配置允许的 JWT 签名算法。
func WithSigningMethods(methods ...string) Option {
	return func(options *options) {
		options.signingMethods = append([]string(nil), methods...)
	}
}

// WithHTTPClient 配置 discovery 和 JWKS 请求使用的 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *options) {
		options.httpClient = httpClient
	}
}

// Authenticator 校验 OIDC ID token。
type Authenticator struct {
	options        *options
	keyfunc        keyfuncV3.Keyfunc
	signingMethods []string
	httpClient     *http.Client
	ownsClient     bool
	cancel         context.CancelFunc
}

var _ engine.Authenticator = (*Authenticator)(nil)

// NewAuthenticator 创建 OIDC 认证器并启动可关闭的 JWKS 刷新。
func NewAuthenticator(opts ...Option) (engine.Authenticator, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if options.issuer == "" {
		return nil, ErrMissingIssuer
	}
	if options.audience == "" {
		return nil, ErrMissingAudience
	}
	parent := options.ctx
	if parent == nil {
		parent = context.Background()
	}
	lifecycleCtx, cancel := context.WithCancel(parent)
	httpClient := options.httpClient
	ownsClient := false
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
		ownsClient = true
	}
	config, err := discover(lifecycleCtx, httpClient, options.issuer)
	if err != nil {
		cancel()
		return nil, err
	}
	signingMethods := allowedSigningMethods(options.signingMethods, config.SigningMethod)
	if len(signingMethods) == 0 {
		cancel()
		return nil, errors.New("oidc: provider has no allowed signing method")
	}
	var keyfunc keyfuncV3.Keyfunc
	keyfunc, err = keyfuncV3.NewDefaultOverrideCtx(lifecycleCtx, []string{config.JWKSURL}, keyfuncV3.Override{
		Client: httpClient,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("oidc: initialize JWKS: %w", err)
	}
	return &Authenticator{
		options:        options,
		keyfunc:        keyfunc,
		signingMethods: signingMethods,
		httpClient:     httpClient,
		ownsClient:     ownsClient,
		cancel:         cancel,
	}, nil
}

// Authenticate 从请求上下文读取 Bearer ID token 并校验。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, _ any) (*engine.AuthClaims, error) {
	token, err := engine.AuthFromMD(ctx, engine.BearerWord, contextType)
	if err != nil {
		return nil, engine.ErrMissingBearerToken
	}
	return a.authenticate(ctx, token)
}

// AuthenticateToken 使用后台 context 校验 ID token。
func (a *Authenticator) AuthenticateToken(token string) (*engine.AuthClaims, error) {
	return a.authenticate(context.Background(), token)
}

// CreateIdentityWithContext 把显式配置的外部 ID token 写入客户端请求。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, _ any) (context.Context, error) {
	token, err := a.CreateIdentity(claims)
	if err != nil {
		return ctx, err
	}
	return engine.MDWithAuth(ctx, engine.BearerWord, token, contextType), nil
}

// CreateIdentity 返回显式配置的外部 ID token。
func (a *Authenticator) CreateIdentity(engine.AuthClaims) (string, error) {
	if a.options.clientToken == "" {
		return "", ErrMissingClientToken
	}
	return a.options.clientToken, nil
}

// Close 停止 JWKS 后台刷新并关闭自建 HTTP 客户端的空闲连接。
func (a *Authenticator) Close() {
	a.cancel()
	if a.ownsClient {
		a.httpClient.CloseIdleConnections()
	}
}

// authenticate 校验 ID token 的签名、issuer、audience 和时效。
func (a *Authenticator) authenticate(ctx context.Context, rawToken string) (*engine.AuthClaims, error) {
	token, err := jwtV5.Parse(
		rawToken,
		a.keyfunc.KeyfuncCtx(ctx),
		jwtV5.WithValidMethods(a.signingMethods),
		jwtV5.WithIssuer(a.options.issuer),
		jwtV5.WithAudience(a.options.audience),
		jwtV5.WithExpirationRequired(),
		jwtV5.WithIssuedAt(),
	)
	if err != nil {
		switch {
		case errors.Is(err, jwtV5.ErrTokenExpired), errors.Is(err, jwtV5.ErrTokenNotValidYet):
			return nil, engine.ErrTokenExpired
		case errors.Is(err, jwtV5.ErrTokenInvalidAudience):
			return nil, engine.ErrInvalidAudience
		case errors.Is(err, jwtV5.ErrTokenInvalidIssuer):
			return nil, engine.ErrInvalidIssuer
		case errors.Is(err, jwtV5.ErrTokenSignatureInvalid):
			return nil, engine.ErrSignTokenFailed
		default:
			return nil, engine.ErrInvalidToken
		}
	}
	claims, ok := token.Claims.(jwtV5.MapClaims)
	if !ok || !token.Valid {
		return nil, engine.ErrInvalidClaims
	}
	authClaims := engine.AuthClaims(claims)
	return &authClaims, nil
}

// discover 读取并校验 OIDC discovery 文档。
func discover(ctx context.Context, httpClient *http.Client, issuer string) (*providerConfig, error) {
	discoveryURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc: create discovery request: %w", err)
	}
	var response *http.Response
	response, err = httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("oidc: fetch discovery: %w", err)
	}
	var body []byte
	body, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("oidc: read discovery: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("oidc: close discovery: %w", closeErr)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc: discovery returned HTTP %d", response.StatusCode)
	}
	config := &providerConfig{}
	if err = json.Unmarshal(body, config); err != nil {
		return nil, fmt.Errorf("oidc: decode discovery: %w", err)
	}
	if config.Issuer != issuer {
		return nil, fmt.Errorf("oidc: discovery issuer %q does not match %q", config.Issuer, issuer)
	}
	if config.JWKSURL == "" {
		return nil, errors.New("oidc: discovery jwks_uri is required")
	}
	return config, nil
}

// allowedSigningMethods 计算调用方、provider 和安全默认列表的交集。
func allowedSigningMethods(configured []string, advertised []string) []string {
	safeSet := make(map[string]struct{}, len(defaultSigningMethods))
	for _, method := range defaultSigningMethods {
		safeSet[method] = struct{}{}
	}
	candidates := configured
	if len(candidates) == 0 {
		candidates = defaultSigningMethods
	}
	allowed := make([]string, 0, len(candidates))
	for _, method := range candidates {
		if _, ok := safeSet[method]; ok {
			allowed = append(allowed, method)
		}
	}
	if len(advertised) == 0 {
		return append([]string(nil), allowed...)
	}
	advertisedSet := make(map[string]struct{}, len(advertised))
	for _, method := range advertised {
		advertisedSet[method] = struct{}{}
	}
	result := make([]string, 0, len(allowed))
	for _, method := range allowed {
		if _, ok := advertisedSet[method]; ok {
			result = append(result, method)
		}
	}
	return result
}
