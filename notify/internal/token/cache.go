package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

// Cache 将平台令牌存入宿主共享缓存，并合并当前进程内的并发刷新。
type Cache struct {
	mutex sync.Mutex
	cache cachekit.Cache
	key   string
}

// New 创建按渠道、配置实例和凭据隔离的令牌缓存键。
func New(cache cachekit.Cache, channelType notify.Type, instanceName string, credentials ...string) (*Cache, error) {
	if cache == nil || channelType == "" || instanceName == "" {
		return nil, errors.New("notify: shared cache, channel type and instance name are required")
	}
	identity := strings.Join(append([]string{string(channelType), instanceName}, credentials...), "\x00")
	keyHash := sha256.Sum256([]byte(identity))
	return &Cache{cache: cache, key: "notify:access-token:" + hex.EncodeToString(keyHash[:])}, nil
}

// Get 返回有效令牌；共享缓存未命中时刷新并按服务端有效期写回。
func (c *Cache) Get(ctx context.Context, refresh func(context.Context) (string, time.Duration, error)) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c.cache.Exists(c.key) {
		value, err := c.cache.Get(c.key)
		if err == nil && value != "" {
			return value, nil
		}
	}
	value, lifetime, err := refresh(ctx)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New("notify platform: token response is empty")
	}
	if lifetime > time.Minute {
		lifetime -= time.Minute
	} else if lifetime > 0 {
		lifetime -= lifetime / 5
	}
	if lifetime <= 0 {
		return "", errors.New("notify platform: token lifetime is invalid")
	}
	if err = c.cache.Set(c.key, value, lifetime); err != nil {
		return "", fmt.Errorf("notify: cache platform token: %w", err)
	}
	return value, nil
}
