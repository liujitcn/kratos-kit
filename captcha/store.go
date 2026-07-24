package captcha

import (
	"errors"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	cachekit "github.com/liujitcn/kratos-kit/cache"
)

var errCacheNil = errors.New("captcha cache is nil")

// cacheStore 适配 kratos-kit/cache.Cache 作为验证码答案存储。
type cacheStore struct {
	cache cachekit.Cache // 实际缓存实现，可为内存缓存或 Redis 缓存
}

// newCacheStore 创建基于缓存的验证码存储实现。
func newCacheStore(cache cachekit.Cache) *cacheStore {
	return &cacheStore{cache: cache}
}

// Set 设置验证码对应的缓存值。
func (e *cacheStore) Set(id string, value string, expiration time.Duration) error {
	// 缓存为空属于构造期配置错误，直接返回明确错误。
	if e == nil || e.cache == nil {
		return errCacheNil
	}
	err := e.cache.Set(id, value, expiration)
	if err != nil {
		log.Error(err.Error())
	}
	return err
}

// Get 获取验证码对应的缓存值。
func (e *cacheStore) Get(id string) string {
	// 读取失败时按未命中处理，避免把缓存错误暴露成验证码通过。
	if e == nil || e.cache == nil {
		return ""
	}
	v, err := e.cache.Get(id)
	if err == nil {
		return v
	}
	return ""
}

// Delete 删除验证码缓存。
func (e *cacheStore) Delete(id string) error {
	// 删除时仍然检查缓存实例，避免 nil 指针掩盖配置问题。
	if e == nil || e.cache == nil {
		return errCacheNil
	}
	return e.cache.Del(id)
}

// Exists 判断验证码缓存是否存在。
func (e *cacheStore) Exists(id string) bool {
	// 缓存不可用时统一按不存在处理。
	if e == nil || e.cache == nil {
		return false
	}
	return e.cache.Exists(id)
}
