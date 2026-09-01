package locker

import (
	"errors"

	"github.com/bsm/redislock"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/locker/redis"
)

// Locker 定义 Redis 分布式锁能力。
type Locker interface {
	Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error)
}

// NewLocker 创建 Redis 分布式锁实例。
func NewLocker(cfg *configv1.Data_Redis) (Locker, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("Redis 配置不能为空")
	}
	return redis.NewRedis(cfg)
}
