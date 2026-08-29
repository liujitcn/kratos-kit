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

// ClusterRedis 表示 Redis 集群缓存客户端。
type ClusterRedis struct {
	client *redis.ClusterClient
}

// NewClusterRedis 创建 Redis 集群缓存实例。
func NewClusterRedis(cfg *configv1.Data_Redis) (*ClusterRedis, func(), error) {
	redisOptions, err := utils.GetClusterRedisOptions(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("redis options failed: %w", err)
	}
	client := redis.NewClusterClient(redisOptions)
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
	if err = client.ForEachShard(context.TODO(), func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	}); err != nil {
		return nil, nil, fmt.Errorf("failed ping redis: %w", err)
	}
	return &ClusterRedis{
			client: client,
		}, func() {
			log.Info("cache cluster-redis cleanup...")
			if client != nil {
				err = client.Close()
				if err != nil {
					log.Error("failed close redis", "error", err)
					return
				}
			}
		}, nil
}

func (s *ClusterRedis) Connect() error {
	return s.client.ForEachShard(context.TODO(), func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
}

func (s *ClusterRedis) DisConnect() error {
	return s.client.Close()
}

func (s *ClusterRedis) Get(key string) (string, error) {
	return s.client.Get(context.TODO(), key).Result()
}

// Incr 原子递增 Redis 集群中的数值键。
func (s *ClusterRedis) Incr(key string) (int64, error) {
	return s.client.Incr(context.TODO(), key).Result()
}

// GetDel 使用 Redis GETDEL 原子读取并删除缓存值。
func (s *ClusterRedis) GetDel(key string) (string, error) {
	return s.client.GetDel(context.TODO(), key).Result()
}

func (s *ClusterRedis) Set(key, value string, expire time.Duration) error {
	return s.client.Set(context.TODO(), key, value, expire).Err()
}

func (s *ClusterRedis) Del(key string) error {
	return s.client.Del(context.TODO(), key).Err()
}

func (s *ClusterRedis) Expire(key string, dur time.Duration) error {
	return s.client.Expire(context.TODO(), key, dur).Err()
}

func (s *ClusterRedis) Exists(key string) bool {
	result, err := s.client.Exists(context.TODO(), key).Result()
	if err != nil {
		return false
	}
	return result != 0
}

func (s *ClusterRedis) HGetAll(key string) (map[string]string, error) {
	return s.client.HGetAll(context.TODO(), key).Result()
}

func (s *ClusterRedis) HGet(key, field string) (string, error) {
	return s.client.HGet(context.TODO(), key, field).Result()
}

func (s *ClusterRedis) HSet(key, field, value string) error {
	return s.client.HSet(context.TODO(), key, field, value).Err()
}

func (s *ClusterRedis) HDel(key, field string) error {
	return s.client.HDel(context.TODO(), key, field).Err()
}

func (s *ClusterRedis) HExists(key, field string) error {
	return s.client.HExists(context.TODO(), key, field).Err()
}
