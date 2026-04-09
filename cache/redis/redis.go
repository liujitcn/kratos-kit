package redis

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/utils"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

// NewRedis 创建单机 Redis 缓存实例。
func NewRedis(cfg *conf.Data_Redis) (*Redis, func(), error) {
	redisOptions, err := utils.GetRedisOptions(cfg)
	if err != nil {
		log.Fatal("redis options failed", err)
		panic(err)
	}
	client := redis.NewClient(redisOptions)
	if client == nil {
		log.Fatalf("failed opening connection to redis")
	}

	// open tracing instrumentation.
	if cfg.GetEnableTracing() {
		if err = redisotel.InstrumentTracing(client); err != nil {
			log.Fatalf("failed open tracing: %s", err.Error())
		}
	}

	// open metrics instrumentation.
	if cfg.GetEnableMetrics() {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			log.Fatalf("failed open metrics: %s", err.Error())
		}
	}

	// 连接
	if _, err = client.Ping(context.TODO()).Result(); err != nil {
		log.Fatalf("failed ping redis: %s", err.Error())
	}
	return &Redis{
			client: client,
		}, func() {
			log.Info("cache redis cleanup...")
			if client != nil {
				err = client.Close()
				if err != nil {
					log.Errorf("failed close redis: %s", err.Error())
					return
				}
			}
		}, nil
}

func (s *Redis) Get(key string) (string, error) {
	return s.client.Get(context.TODO(), key).Result()
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
