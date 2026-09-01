package redis

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/cache/store"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// ClusterRedis 表示 Redis 集群缓存客户端。
type ClusterRedis struct {
	client *redis.ClusterClient
	metaMu sync.RWMutex
	meta   map[string]entryMeta
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
			meta:   make(map[string]entryMeta),
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

// List 返回 Redis 集群中支持的缓存条目及其运行时元数据。
func (s *ClusterRedis) List() ([]store.Entry, error) {
	keySet := make(map[string]struct{})
	err := s.client.ForEachShard(context.TODO(), func(ctx context.Context, shard *redis.Client) error {
		keys, err := scanKeys(shard)
		if err != nil {
			return err
		}
		for _, key := range keys {
			keySet[key] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]store.Entry, 0, len(keys))
	for _, key := range keys {
		kind, err := s.client.Type(context.TODO(), key).Result()
		if err != nil || (kind != "string" && kind != "hash") {
			continue
		}
		ttlSeconds, err := s.client.TTL(context.TODO(), key).Result()
		if err != nil {
			continue
		}
		now := time.Now()
		meta := s.getMeta(key, now)
		entry := store.Entry{Key: key, Type: kind, TTL: redisTTL(ttlSeconds), CreatedAt: meta.createdAt, UpdatedAt: meta.updatedAt}
		if ttlSeconds >= 0 {
			entry.ExpiresAt = now.Add(ttlSeconds)
		}
		if kind == "string" {
			entry.Value, err = s.client.Get(context.TODO(), key).Result()
		} else {
			entry.Fields, err = s.client.HGetAll(context.TODO(), key).Result()
		}
		if err == nil {
			entries = append(entries, entry)
		}
	}
	return entries, nil
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
	value, err := s.client.Incr(context.TODO(), key).Result()
	if err == nil {
		s.recordMeta(key)
	}
	return value, err
}

// GetDel 使用 Redis GETDEL 原子读取并删除缓存值。
func (s *ClusterRedis) GetDel(key string) (string, error) {
	value, err := s.client.GetDel(context.TODO(), key).Result()
	if err == nil {
		s.deleteMeta(key)
	}
	return value, err
}

func (s *ClusterRedis) Set(key, value string, expire time.Duration) error {
	err := s.client.Set(context.TODO(), key, value, expire).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *ClusterRedis) Del(key string) error {
	err := s.client.Del(context.TODO(), key).Err()
	if err == nil {
		s.deleteMeta(key)
	}
	return err
}

func (s *ClusterRedis) Expire(key string, dur time.Duration) error {
	err := s.client.Expire(context.TODO(), key, dur).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
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
	err := s.client.HSet(context.TODO(), key, field, value).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *ClusterRedis) HDel(key, field string) error {
	err := s.client.HDel(context.TODO(), key, field).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *ClusterRedis) HExists(key, field string) error {
	return s.client.HExists(context.TODO(), key, field).Err()
}

func (s *ClusterRedis) recordMeta(key string) {
	now := time.Now()
	s.metaMu.Lock()
	item, ok := s.meta[key]
	if !ok {
		item.createdAt = now
	}
	item.updatedAt = now
	s.meta[key] = item
	s.metaMu.Unlock()
}

func (s *ClusterRedis) getMeta(key string, now time.Time) entryMeta {
	s.metaMu.Lock()
	item, ok := s.meta[key]
	if !ok {
		item = entryMeta{createdAt: now, updatedAt: now}
		s.meta[key] = item
	}
	s.metaMu.Unlock()
	return item
}

func (s *ClusterRedis) deleteMeta(key string) {
	s.metaMu.Lock()
	delete(s.meta, key)
	s.metaMu.Unlock()
}
