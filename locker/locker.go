package locker

import (
	"errors"

	"github.com/bsm/redislock"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/locker/redis"
)

type Locker interface {
	Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error)
}

func NewLocker(cfg *conf.Data_Redis) (Locker, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("redisConf is null")
	}
	return redis.NewRedis(cfg)
}
