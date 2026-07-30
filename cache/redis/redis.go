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
)

type Redis struct {
	client *redis.Client
}

// NewRedis 创建单机 Redis 缓存实例。
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
			log.Info("cache redis cleanup...")
			if client != nil {
				err = client.Close()
				if err != nil {
					log.Error("failed close redis", "error", err)
					return
				}
			}
		}, nil
}

func (s *Redis) Get(key string) (string, error) {
	return s.client.Get(context.TODO(), key).Result()
}

// GetDel 使用 Redis GETDEL 原子读取并删除缓存值。
func (s *Redis) GetDel(key string) (string, error) {
	return s.client.GetDel(context.TODO(), key).Result()
}

func (s *Redis) Set(key, value string, expire time.Duration) error {
	return s.client.Set(context.TODO(), key, value, expire).Err()
}

func (s *Redis) Del(key string) error {
	return s.client.Del(context.TODO(), key).Err()
}

func (s *Redis) Expire(key string, dur time.Duration) error {
	return s.client.Expire(context.TODO(), key, dur).Err()
}

func (s *Redis) Exists(key string) bool {
	result, err := s.client.Exists(context.TODO(), key).Result()
	if err != nil {
		return false
	}
	return result != 0
}

func (s *Redis) HGetAll(key string) (map[string]string, error) {
	return s.client.HGetAll(context.TODO(), key).Result()
}

func (s *Redis) HGet(key, field string) (string, error) {
	return s.client.HGet(context.TODO(), key, field).Result()
}

func (s *Redis) HSet(key, field, value string) error {
	return s.client.HSet(context.TODO(), key, field, value).Err()
}

func (s *Redis) HDel(key, field string) error {
	return s.client.HDel(context.TODO(), key, field).Err()
}

func (s *Redis) HExists(key, field string) error {
	return s.client.HExists(context.TODO(), key, field).Err()
}
