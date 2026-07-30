// Package hmac 提供绑定请求内容并防重放的 HMAC-SHA256 认证器。
package hmac

import (
	"context"
	cryptohmac "crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/proto"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

var (
	// ErrMissingSecret 表示没有配置 subject 对应的 HMAC 密钥。
	ErrMissingSecret = errors.New("hmac: secret is required")
	// ErrMissingSubject 表示声明中没有提供 HMAC key ID。
	ErrMissingSubject = errors.New("hmac: subject is required")
	// ErrMissingTransport 表示请求上下文中没有可用于签名的 Kratos transport。
	ErrMissingTransport = errors.New("hmac: kratos transport is required")
	// ErrInvalidTransport 表示 Kratos transport 缺少请求签名所需的信息。
	ErrInvalidTransport = errors.New("hmac: invalid kratos transport")
	// ErrTokenReplayed 表示相同 HMAC nonce 已经被消费。
	ErrTokenReplayed = errors.New("hmac: token has already been used")
)

// SecretResolver 根据 key ID 解析 HMAC 密钥。
type SecretResolver func(keyID string) (string, bool)

// NonceGenerator 创建单次请求使用的随机 nonce。
type NonceGenerator func() (string, error)

// ReplayStore 原子记录 nonce，返回当前 nonce 是否首次使用。
type ReplayStore interface {
	// Consume 原子消费 nonce，并在 expiresAt 后允许清理记录。
	Consume(context.Context, string, time.Time) (bool, error)
}

// MemoryReplayStore 是并发安全的进程内 nonce 存储。
type MemoryReplayStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
	now     func() time.Time
}

type httpRequestTransporter interface {
	Request() *http.Request
}

// NewMemoryReplayStore 创建进程内 nonce 存储。
func NewMemoryReplayStore() *MemoryReplayStore {
	return newMemoryReplayStore(time.Now)
}

// Consume 原子消费 nonce，并清理已经过期的记录。
func (m *MemoryReplayStore) Consume(ctx context.Context, key string, expiresAt time.Time) (bool, error) {
	err := ctx.Err()
	if err != nil {
		return false, err
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for nonce, expiration := range m.entries {
		if now.After(expiration) {
			delete(m.entries, nonce)
		}
	}
	if expiration, ok := m.entries[key]; ok && !now.After(expiration) {
		return false, nil
	}
	m.entries[key] = expiresAt
	return true, nil
}

// Option 配置 HMAC 认证器。
type Option func(*options)

type options struct {
	secrets        map[string]string
	resolver       SecretResolver
	maxSkew        time.Duration
	now            func() time.Time
	nonceGenerator NonceGenerator
	replayStore    ReplayStore
}

// WithSecret 添加一个 key ID 和 HMAC 密钥。
func WithSecret(keyID string, secret string) Option {
	return func(options *options) {
		if options.secrets == nil {
			options.secrets = make(map[string]string)
		}
		options.secrets[keyID] = secret
	}
}

// WithSecrets 配置 key ID 和 HMAC 密钥集合。
func WithSecrets(secrets map[string]string) Option {
	return func(options *options) {
		options.secrets = make(map[string]string, len(secrets))
		for keyID, secret := range secrets {
			options.secrets[keyID] = secret
		}
	}
}

// WithSecretResolver 配置外部 HMAC 密钥解析函数。
func WithSecretResolver(resolver SecretResolver) Option {
	return func(options *options) {
		options.resolver = resolver
	}
}

// WithMaxSkew 配置允许的最大时钟偏差和 nonce 保留时间。
func WithMaxSkew(maxSkew time.Duration) Option {
	return func(options *options) {
		options.maxSkew = maxSkew
	}
}

// WithNonceGenerator 配置 nonce 创建函数。
func WithNonceGenerator(generator NonceGenerator) Option {
	return func(options *options) {
		options.nonceGenerator = generator
	}
}

// WithReplayStore 配置 nonce 防重放存储。
func WithReplayStore(store ReplayStore) Option {
	return func(options *options) {
		options.replayStore = store
	}
}

// withNow 为测试配置当前时间函数。
func withNow(now func() time.Time) Option {
	return func(options *options) {
		options.now = now
	}
}

// Authenticator 校验绑定 Kratos 请求的 HMAC Bearer 令牌。
type Authenticator struct {
	options *options
}

var _ engine.RequestAuthenticator = (*Authenticator)(nil)
var _ engine.ContextIdentityCreator = (*Authenticator)(nil)
var _ engine.AuthenticatorCloser = (*Authenticator)(nil)

// NewAuthenticator 创建 HMAC 认证器。
func NewAuthenticator(opts ...Option) (*Authenticator, error) {
	options := &options{
		maxSkew:        5 * time.Minute,
		now:            time.Now,
		nonceGenerator: newNonce,
	}
	for _, option := range opts {
		option(options)
	}
	if len(options.secrets) == 0 && options.resolver == nil {
		return nil, ErrMissingSecret
	}
	if options.replayStore == nil {
		options.replayStore = newMemoryReplayStore(options.now)
	}
	return &Authenticator{options: options}, nil
}

// Authenticate 从请求上下文读取 HMAC Bearer 令牌，校验请求签名并消费 nonce。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, request any) (*engine.AuthClaims, error) {
	token, err := engine.AuthFromMD(ctx, engine.BearerWord, contextType)
	if err != nil {
		return nil, engine.ErrMissingBearerToken
	}
	var keyID string
	var timestamp string
	var nonce string
	var signature string
	keyID, timestamp, nonce, signature, err = parseToken(token)
	if err != nil {
		return nil, engine.ErrInvalidToken
	}
	var unixTimestamp int64
	unixTimestamp, err = strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, engine.ErrInvalidToken
	}
	tokenTime := time.Unix(unixTimestamp, 0)
	skew := a.options.now().Sub(tokenTime)
	if skew < -a.options.maxSkew || skew > a.options.maxSkew {
		return nil, engine.ErrTokenExpired
	}
	secret, ok := a.secret(keyID)
	if !ok {
		return nil, engine.ErrUnauthenticated
	}
	var canonical string
	canonical, err = canonicalRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	expected := sign(secret, keyID, timestamp, nonce, canonical)
	if !subtleEqual(signature, expected) {
		return nil, engine.ErrUnauthenticated
	}
	var consumed bool
	consumed, err = a.options.replayStore.Consume(ctx, keyID+"\x00"+nonce, tokenTime.Add(a.options.maxSkew))
	if err != nil {
		return nil, fmt.Errorf("hmac: consume nonce: %w", err)
	}
	if !consumed {
		return nil, ErrTokenReplayed
	}
	claims := engine.AuthClaims{engine.ClaimFieldSubject: keyID}
	return &claims, nil
}

// CreateIdentityWithContext 创建绑定当前请求的 HMAC 令牌并写入客户端请求上下文。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, request any) (context.Context, error) {
	keyID, err := claims.GetSubject()
	if err != nil {
		return ctx, err
	}
	if keyID == "" {
		return ctx, ErrMissingSubject
	}
	secret, ok := a.secret(keyID)
	if !ok {
		return ctx, ErrMissingSecret
	}
	var canonical string
	canonical, err = canonicalRequest(ctx, request)
	if err != nil {
		return ctx, err
	}
	var nonce string
	nonce, err = a.options.nonceGenerator()
	if err != nil {
		return ctx, fmt.Errorf("hmac: create nonce: %w", err)
	}
	if nonce == "" || strings.Contains(nonce, ".") {
		return ctx, engine.ErrInvalidToken
	}
	timestamp := strconv.FormatInt(a.options.now().Unix(), 10)
	token := keyID + "." + timestamp + "." + nonce + "." + sign(secret, keyID, timestamp, nonce, canonical)
	return engine.MDWithAuth(ctx, engine.BearerWord, token, contextType), nil
}

// Close 释放认证器资源。
func (a *Authenticator) Close() {}

// secret 解析 key ID 对应的 HMAC 密钥。
func (a *Authenticator) secret(keyID string) (string, bool) {
	if a.options.resolver != nil {
		return a.options.resolver(keyID)
	}
	secret, ok := a.options.secrets[keyID]
	return secret, ok
}

// newMemoryReplayStore 使用指定时钟创建进程内 nonce 存储。
func newMemoryReplayStore(now func() time.Time) *MemoryReplayStore {
	return &MemoryReplayStore{
		entries: make(map[string]time.Time),
		now:     now,
	}
}

// canonicalRequest 构造方法、路径、查询参数和请求体摘要组成的规范请求。
func canonicalRequest(ctx context.Context, request any) (string, error) {
	method, requestPath, rawQuery, err := requestTarget(ctx)
	if err != nil {
		return "", err
	}
	var digest string
	digest, err = requestDigest(request)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{method, requestPath, rawQuery, digest}, "\n"), nil
}

// requestTarget 从 Kratos 3.0 transport 提取规范请求目标。
func requestTarget(ctx context.Context) (string, string, string, error) {
	transporter, ok := transport.FromServerContext(ctx)
	if !ok {
		transporter, ok = transport.FromClientContext(ctx)
	}
	if !ok {
		return "", "", "", ErrMissingTransport
	}
	switch transporter.Kind() {
	case transport.KindHTTP:
		httpTransporter, matched := transporter.(httpRequestTransporter)
		if !matched || httpTransporter.Request() == nil || httpTransporter.Request().URL == nil {
			return "", "", "", ErrInvalidTransport
		}
		httpRequest := httpTransporter.Request()
		requestPath := httpRequest.URL.EscapedPath()
		if requestPath == "" {
			requestPath = "/"
		}
		return httpRequest.Method, requestPath, httpRequest.URL.Query().Encode(), nil
	case transport.KindGRPC:
		if transporter.Operation() == "" {
			return "", "", "", ErrInvalidTransport
		}
		return http.MethodPost, transporter.Operation(), "", nil
	default:
		return "", "", "", ErrInvalidTransport
	}
}

// requestDigest 以确定性编码计算业务请求摘要。
func requestDigest(request any) (string, error) {
	var data []byte
	var err error
	if message, ok := request.(proto.Message); ok {
		data, err = proto.MarshalOptions{Deterministic: true}.Marshal(message)
	} else {
		data, err = json.Marshal(request)
	}
	if err != nil {
		return "", fmt.Errorf("hmac: encode request: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

// parseToken 解析 keyID.timestamp.nonce.signature 格式的 HMAC 令牌。
func parseToken(token string) (string, string, string, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return "", "", "", "", engine.ErrInvalidToken
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

// sign 计算请求绑定的十六进制 HMAC-SHA256 签名。
func sign(secret string, keyID string, timestamp string, nonce string, canonical string) string {
	hash := cryptohmac.New(sha256.New, []byte(secret))
	// hash.Hash.Write 按接口约定始终返回 nil 错误。
	_, _ = hash.Write([]byte(keyID + "." + timestamp + "." + nonce + "\n" + canonical))
	return hex.EncodeToString(hash.Sum(nil))
}

// subtleEqual 使用恒定时间比较两个签名。
func subtleEqual(actual string, expected string) bool {
	return cryptohmac.Equal([]byte(actual), []byte(expected))
}

// newNonce 创建密码学安全的十六进制 nonce。
func newNonce() (string, error) {
	value := make([]byte, 16)
	_, err := rand.Read(value)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
