package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"google.golang.org/grpc/metadata"
)

const (
	defaultHeaderName = "X-Session-Id"
	defaultTTL        = 24 * time.Hour
)

var (
	// ErrMissingSessionID 表示请求中没有会话 ID。
	ErrMissingSessionID = errors.New("session: session id is required")
	// ErrSessionNotFound 表示会话不存在或已经失效。
	ErrSessionNotFound = errors.New("session: session not found")
)

// Store 定义会话声明的存取能力。
type Store interface {
	// Get 读取会话声明。
	Get(context.Context, string) (map[string]any, bool, error)
	// Set 按指定有效期保存会话声明；sessionID 为空时由存储生成。
	Set(context.Context, string, map[string]any, time.Duration) (string, error)
	// Delete 删除会话。
	Delete(context.Context, string) error
}

// MemoryStore 是并发安全的内存会话存储。
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]memorySession
	now      func() time.Time
}

type memorySession struct {
	claims    map[string]any
	expiresAt time.Time
}

// MemoryStoreOption 配置内存会话存储。
type MemoryStoreOption func(*MemoryStore)

// WithMemoryStoreClock 配置内存会话存储使用的时钟。
func WithMemoryStoreClock(now func() time.Time) MemoryStoreOption {
	return func(store *MemoryStore) {
		if now != nil {
			store.now = now
		}
	}
}

// NewMemoryStore 创建内存会话存储。
func NewMemoryStore(opts ...MemoryStoreOption) *MemoryStore {
	store := &MemoryStore{
		sessions: make(map[string]memorySession),
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// Get 读取会话声明，并原子删除已经过期的会话。
func (m *MemoryStore) Get(_ context.Context, sessionID string) (map[string]any, bool, error) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok && !m.now().Before(session.expiresAt) {
		delete(m.sessions, sessionID)
		ok = false
	}
	m.mu.Unlock()
	if !ok {
		return nil, false, nil
	}
	return cloneClaims(session.claims), true, nil
}

// Set 保存会话声明。
func (m *MemoryStore) Set(_ context.Context, sessionID string, claims map[string]any, ttl time.Duration) (string, error) {
	var err error
	if sessionID == "" {
		sessionID, err = newSessionID()
		if err != nil {
			return "", err
		}
	}
	m.mu.Lock()
	m.sessions[sessionID] = memorySession{
		claims:    cloneClaims(claims),
		expiresAt: m.now().Add(ttl),
	}
	m.mu.Unlock()
	return sessionID, nil
}

// Delete 删除会话。
func (m *MemoryStore) Delete(_ context.Context, sessionID string) error {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	return nil
}

// Option 配置会话认证器。
type Option func(*options)

type options struct {
	store      Store
	headerName string
	ttl        time.Duration
}

// WithStore 配置会话存储。
func WithStore(store Store) Option {
	return func(options *options) {
		options.store = store
	}
}

// WithHeaderName 配置承载会话 ID 的 HTTP/gRPC 头名称。
func WithHeaderName(headerName string) Option {
	return func(options *options) {
		options.headerName = headerName
	}
}

// WithTTL 配置新建会话的有效期。
func WithTTL(ttl time.Duration) Option {
	return func(options *options) {
		if ttl > 0 {
			options.ttl = ttl
		}
	}
}

// Authenticator 校验会话 ID。
type Authenticator struct {
	options *options
}

var _ engine.Authenticator = (*Authenticator)(nil)

// NewAuthenticator 创建会话认证器。
func NewAuthenticator(opts ...Option) (engine.Authenticator, error) {
	options := &options{
		headerName: defaultHeaderName,
		ttl:        defaultTTL,
	}
	for _, option := range opts {
		option(options)
	}
	if options.store == nil {
		options.store = NewMemoryStore()
	}
	return &Authenticator{options: options}, nil
}

// Authenticate 从请求上下文读取会话 ID 并校验。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, _ any) (*engine.AuthClaims, error) {
	sessionID := sessionIDFromContext(ctx)
	if sessionID == "" {
		sessionID = sessionIDFromTransport(ctx, contextType, a.options.headerName)
	}
	if sessionID == "" {
		return nil, ErrMissingSessionID
	}
	return a.authenticate(ctx, sessionID)
}

// AuthenticateToken 校验会话 ID。
func (a *Authenticator) AuthenticateToken(sessionID string) (*engine.AuthClaims, error) {
	return a.authenticate(context.Background(), sessionID)
}

// CreateIdentityWithContext 创建会话并把 ID 写入客户端请求上下文。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, _ any) (context.Context, error) {
	sessionID, err := a.options.store.Set(ctx, "", map[string]any(claims), a.options.ttl)
	if err != nil {
		return ctx, err
	}
	ctx = context.WithValue(ctx, sessionIDKey{}, sessionID)
	return withSessionID(ctx, contextType, a.options.headerName, sessionID), nil
}

// CreateIdentity 创建会话并返回 ID。
func (a *Authenticator) CreateIdentity(claims engine.AuthClaims) (string, error) {
	return a.options.store.Set(context.Background(), "", map[string]any(claims), a.options.ttl)
}

// Close 释放认证器资源。
func (a *Authenticator) Close() {}

// Delete 删除指定会话。
func (a *Authenticator) Delete(ctx context.Context, sessionID string) error {
	return a.options.store.Delete(ctx, sessionID)
}

type sessionIDKey struct{}

// ContextWithSessionID 把会话 ID 写入上下文。
func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFromContext 从上下文读取会话 ID。
func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID := sessionIDFromContext(ctx)
	return sessionID, sessionID != ""
}

// authenticate 从存储读取并返回会话声明。
func (a *Authenticator) authenticate(ctx context.Context, sessionID string) (*engine.AuthClaims, error) {
	if sessionID == "" {
		return nil, ErrMissingSessionID
	}
	claims, found, err := a.options.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrSessionNotFound
	}
	authClaims := engine.AuthClaims(claims)
	return &authClaims, nil
}

// sessionIDFromContext 从包内上下文值读取会话 ID。
func sessionIDFromContext(ctx context.Context) string {
	sessionID, _ := ctx.Value(sessionIDKey{}).(string)
	return sessionID
}

// sessionIDFromTransport 从 Kratos transport 或原生 gRPC metadata 读取会话 ID。
func sessionIDFromTransport(ctx context.Context, contextType engine.ContextType, headerName string) string {
	if contextType == engine.ContextTypeKratosMetaData {
		if transporter, ok := transport.FromServerContext(ctx); ok {
			return transporter.RequestHeader().Get(headerName)
		}
	}
	incoming, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := incoming.Get(headerName)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// withSessionID 把会话 ID 写入 Kratos transport 或原生 gRPC metadata。
func withSessionID(ctx context.Context, contextType engine.ContextType, headerName string, sessionID string) context.Context {
	if contextType == engine.ContextTypeKratosMetaData {
		if transporter, ok := transport.FromClientContext(ctx); ok {
			transporter.RequestHeader().Set(headerName, sessionID)
		}
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, headerName, sessionID)
}

// newSessionID 创建密码学安全的会话 ID。
func newSessionID() (string, error) {
	value := make([]byte, 32)
	_, err := rand.Read(value)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// cloneClaims 递归复制声明中的 map、slice 和 array，并保留命名类型。
func cloneClaims(claims map[string]any) map[string]any {
	if claims == nil {
		return nil
	}
	return cloneValue(reflect.ValueOf(claims)).Interface().(map[string]any)
}

// cloneValue 递归复制可变的复合值。
func cloneValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(cloneValue(value.Elem()))
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			cloned.SetMapIndex(cloneValue(iterator.Key()), cloneValue(iterator.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			cloned.Index(index).Set(cloneValue(value.Index(index)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for index := range value.Len() {
			cloned.Index(index).Set(cloneValue(value.Index(index)))
		}
		return cloned
	default:
		return value
	}
}
