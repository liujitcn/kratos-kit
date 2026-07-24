package locker

import (
	"errors"

	"github.com/bsm/redislock"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/locker/redis"
)

type Locker interface {
	Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error)
}

func NewLocker(cfg *configv1.Data_Redis) (Locker, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("redisConf is null")
	}
	return redis.NewRedis(cfg)
}
