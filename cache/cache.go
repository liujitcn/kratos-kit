package cache

import (
	"errors"
	"time"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/cache/memory"
	"github.com/liujitcn/kratos-kit/cache/redis"
)

type Cache interface {
	Get(key string) (string, error)
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

func NewCache(cfg *conf.Data_Redis) (Cache, func(), error) {
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
