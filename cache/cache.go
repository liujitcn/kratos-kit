package cache

import (
	"errors"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/cache/memory"
	"github.com/liujitcn/kratos-kit/cache/redis"
	"github.com/liujitcn/kratos-kit/cache/store"
)

type Cache interface {
	// List 返回当前缓存中的条目及其运行时元数据。
	List() ([]Entry, error)
	Get(key string) (string, error)
	// Incr 原子递增字符串数值键并返回递增后的值。
	Incr(key string) (int64, error)
	// GetDel 原子读取并删除缓存值。
	GetDel(key string) (string, error)
	Set(key string, value string, expire time.Duration) error
	Del(key string) error
	Expire(key string, dur time.Duration) error
	Exists(key string) bool

	HGetAll(key string) (map[string]string, error)
	HGet(key, field string) (string, error)
	HSet(key, field, value string) error
	HDel(key string, field string) error
	HExists(key, field string) error
}

// Entry 表示一个缓存条目的只读快照。
type Entry = store.Entry

func NewCache(cfg *configv1.Data_Redis) (Cache, func(), error) {
	var cache Cache
	var cleanup func()
	var err error
	if cfg == nil {
		cache, cleanup, err = memory.NewMemory()
	} else {
		if len(cfg.Addr) == 0 {
			err = errors.New("addr is null")
		} else if len(cfg.Addr) == 1 {
			cache, cleanup, err = redis.NewRedis(cfg)
		} else {
			cache, cleanup, err = redis.NewClusterRedis(cfg)
		}
	}
	if err != nil {
		return nil, cleanup, err
	}
	return cache, cleanup, nil
}
