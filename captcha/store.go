package captcha

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/mojocn/base64Captcha"
)

type cacheStore struct {
	cache      cache.Cache
	expiration time.Duration
}

// NewCacheStore 创建基于缓存的验证码存储实现。
func NewCacheStore(cache cache.Cache, expiration time.Duration) base64Captcha.Store {
	s := new(cacheStore)
	s.cache = cache
	s.expiration = expiration
	return s
}

// Set 设置验证码对应的缓存值。
func (e *cacheStore) Set(id string, value string) error {
	err := e.cache.Set(id, value, e.expiration)
	if err != nil {
		log.Error(err.Error())
	}
	return err
}

// Get 获取验证码对应的缓存值。
func (e *cacheStore) Get(id string, clear bool) string {
	v, err := e.cache.Get(id)
	if err == nil {
		if clear {
			_ = e.cache.Del(id)
		}
		return v
	}
	return ""
}

// Verify 校验验证码答案。
func (e *cacheStore) Verify(id, answer string, clear bool) bool {
	return e.Get(id, clear) == answer
}
