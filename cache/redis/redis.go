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

type Redis struct {
	client *redis.Client
	metaMu sync.RWMutex
	meta   map[string]entryMeta
}

type entryMeta struct {
	createdAt time.Time
	updatedAt time.Time
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
			meta:   make(map[string]entryMeta),
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

// List 返回 Redis 中支持的缓存条目及其运行时元数据。
func (s *Redis) List() ([]store.Entry, error) {
	keys, err := scanKeys(s.client)
	if err != nil {
		return nil, err
	}
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
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

func (s *Redis) Get(key string) (string, error) {
	return s.client.Get(context.TODO(), key).Result()
}

// Incr 原子递增 Redis 中的数值键。
func (s *Redis) Incr(key string) (int64, error) {
	value, err := s.client.Incr(context.TODO(), key).Result()
	if err == nil {
		s.recordMeta(key)
	}
	return value, err
}

// GetDel 使用 Redis GETDEL 原子读取并删除缓存值。
func (s *Redis) GetDel(key string) (string, error) {
	value, err := s.client.GetDel(context.TODO(), key).Result()
	if err == nil {
		s.deleteMeta(key)
	}
	return value, err
}

func (s *Redis) Set(key, value string, expire time.Duration) error {
	err := s.client.Set(context.TODO(), key, value, expire).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *Redis) Del(key string) error {
	err := s.client.Del(context.TODO(), key).Err()
	if err == nil {
		s.deleteMeta(key)
	}
	return err
}

func (s *Redis) Expire(key string, dur time.Duration) error {
	err := s.client.Expire(context.TODO(), key, dur).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
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
	err := s.client.HSet(context.TODO(), key, field, value).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *Redis) HDel(key, field string) error {
	err := s.client.HDel(context.TODO(), key, field).Err()
	if err == nil {
		s.recordMeta(key)
	}
	return err
}

func (s *Redis) HExists(key, field string) error {
	return s.client.HExists(context.TODO(), key, field).Err()
}

func scanKeys(client *redis.Client) ([]string, error) {
	keys := make([]string, 0)
	var cursor uint64
	var err error
	for {
		var batch []string
		batch, cursor, err = client.Scan(context.TODO(), cursor, "*", 200).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func redisTTL(ttl time.Duration) time.Duration {
	if ttl < 0 {
		return -1
	}
	return ttl
}

func (s *Redis) recordMeta(key string) {
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

func (s *Redis) getMeta(key string, now time.Time) entryMeta {
	s.metaMu.Lock()
	item, ok := s.meta[key]
	if !ok {
		item = entryMeta{createdAt: now, updatedAt: now}
		s.meta[key] = item
	}
	s.metaMu.Unlock()
	return item
}

func (s *Redis) deleteMeta(key string) {
	s.metaMu.Lock()
	delete(s.meta, key)
	s.metaMu.Unlock()
}
