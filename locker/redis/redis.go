package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/bsm/redislock"
)

// Redis 是基于 Redis 的分布式锁实现。
type Redis struct {
	client redis.UniversalClient
	mutex  *redislock.Client
}

// NewRedis 创建 Redis 分布式锁实例。
func NewRedis(cfg *configv1.Data_Redis) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("Redis 配置不能为空")
	}
	if len(cfg.GetAddr()) == 0 || cfg.GetAddr()[0] == "" {
		return nil, nil, errors.New("Redis 地址不能为空")
	}
	redisOptions, err := utils.GetUniversalOptions(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("redis options failed: %w", err)
	}
	client := redis.NewUniversalClient(redisOptions)
	if client == nil {
		return nil, nil, fmt.Errorf("failed opening connection to redis")
	}
	cleanup := func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Error("failed close redis", "error", closeErr)
		}
	}

	// open tracing instrumentation.
	if cfg.GetEnableTracing() {
		if err = redisotel.InstrumentTracing(client); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed open tracing: %w", err)
		}
	}

	// open metrics instrumentation.
	if cfg.GetEnableMetrics() {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed open metrics: %w", err)
		}
	}

	// 启动时主动探测 Redis；锁初始化失败由上层决定是否降级到单机模式。
	if _, err = client.Ping(context.Background()).Result(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed ping redis: %w", err)
	}
	return &Redis{
		client: client,
		mutex:  redislock.New(client),
	}, cleanup, nil
}

// Lock 尝试取得指定 key 的 Redis 分布式锁。
func (r *Redis) Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error) {
	if r == nil || r.client == nil || r.mutex == nil {
		return nil, errors.New("Redis 分布式锁未初始化")
	}
	if key == "" {
		return nil, errors.New("锁 key 不能为空")
	}
	if ttl <= 0 {
		return nil, errors.New("锁 TTL 必须大于零")
	}
	return r.mutex.Obtain(context.Background(), key, time.Duration(ttl)*time.Second, options)
}
