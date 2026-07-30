package oauth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/liujitcn/go-utils/id"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/oauth/provider"
)

const defaultStateTTL = 10 * time.Minute

// StatePayload 表示 OAuth state 关联的业务载荷。
type StatePayload struct {
	Provider    provider.Type          `json:"provider,omitempty"`
	Scene       string                 `json:"scene,omitempty"`
	RedirectURL string                 `json:"redirect_url,omitempty"`
	Extra       map[string]string      `json:"extra,omitempty"`
	PKCE        provider.PKCEChallenge `json:"pkce,omitempty"`
	CreatedAt   int64                  `json:"created_at,omitempty"`
}

// NewStateWithPKCE 生成 PKCE 参数与 OAuth state，并将业务载荷写入缓存。
func NewStateWithPKCE(store cache.Cache, payload StatePayload, ttl time.Duration) (string, provider.PKCEChallenge, error) {
	pkce := provider.GeneratePKCE()
	payload.PKCE = pkce
	state, err := NewState(store, payload, ttl)
	if err != nil {
		return "", provider.PKCEChallenge{}, err
	}
	return state, pkce, nil
}

// NewState 生成 OAuth state，并将业务载荷写入缓存。
func NewState(store cache.Cache, payload StatePayload, ttl time.Duration) (string, error) {
	if store == nil {
		return "", fmt.Errorf("oauth: state cache is nil")
	}
	if ttl <= 0 {
		ttl = defaultStateTTL
	}
	state := id.NewGUIDv7NoHyphen()
	if payload.CreatedAt == 0 {
		payload.CreatedAt = time.Now().Unix()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	err = store.Set(stateKey(state), string(body), ttl)
	if err != nil {
		return "", err
	}
	return state, nil
}

// VerifyState 校验 OAuth state，读取成功后删除缓存，保证 state 只能使用一次。
func VerifyState(store cache.Cache, state string) (*StatePayload, error) {
	if store == nil || state == "" {
		return nil, provider.ErrInvalidState
	}
	value, err := store.GetDel(stateKey(state))
	if err != nil {
		return nil, provider.ErrInvalidState
	}

	var payload StatePayload
	err = json.Unmarshal([]byte(value), &payload)
	if err != nil {
		return nil, provider.ErrInvalidState
	}
	return &payload, nil
}

// stateKey 生成 OAuth state 缓存键。
func stateKey(state string) string {
	return "oauth:state:" + state
}
