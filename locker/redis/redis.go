package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/bsm/redislock"
)

type Redis struct {
	client *redis.Client
	mutex  *redislock.Client
}

// NewRedis 创建 Redis 分布式锁实例。
func NewRedis(cfg *configv1.Data_Redis) (*Redis, func(), error) {
	redisOptions, err := utils.GetRedisOptions(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("redis options failed: %w", err)
	}
	client := redis.NewClient(redisOptions)
	if client == nil {
		return nil, nil, fmt.Errorf("failed opening connection to redis")
	}

	// open tracing instrumentation.
	if cfg.GetEnableTracing() {
		if err = redisotel.InstrumentTracing(client); err != nil {
			return nil, nil, fmt.Errorf("failed open tracing: %w", err)
		}
	}

	// open metrics instrumentation.
	if cfg.GetEnableMetrics() {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			return nil, nil, fmt.Errorf("failed open metrics: %w", err)
		}
	}

	// 连接
	if _, err = client.Ping(context.TODO()).Result(); err != nil {
		return nil, nil, fmt.Errorf("failed ping redis: %w", err)
	}
	return &Redis{
			client: client,
		}, func() {
			if client != nil {
				err = client.Close()
				if err != nil {
					log.Error("failed close redis", "error", err)
					return
				}
			}
		}, nil
}

func (r *Redis) Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error) {
	if r.mutex == nil {
		r.mutex = redislock.New(r.client)
	}
	return r.mutex.Obtain(context.TODO(), key, time.Duration(ttl)*time.Second, options)
}
