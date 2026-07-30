package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	cacheStore "github.com/liujitcn/kratos-kit/cache/store"
)

// StoreOption 配置 Redis Store。
type StoreOption func(*storeConfig)

type storeConfig struct {
	keyPrefix string
}

// WithStoreKeyPrefix 设置 Redis Store 的缓存键前缀。
func WithStoreKeyPrefix(prefix string) StoreOption {
	return func(c *storeConfig) {
		c.keyPrefix = prefix
	}
}

// NewStore 创建基于 Redis UniversalClient 的现代缓存实现。
// Store 不接管 client 生命周期，Close 不会关闭底层 Redis 客户端。
func NewStore(client goredis.UniversalClient, opts ...StoreOption) *Store {
	cfg := &storeConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Store{
		client: client,
		cfg:    cfg,
	}
}

// Store 是支持 context、GetDel、SetNX 和批量操作的 Redis 缓存。
type Store struct {
	client goredis.UniversalClient
	cfg    *storeConfig
}

var _ cacheStore.Store = (*Store)(nil)

// Get 获取缓存值。
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, s.prefix(key)).Bytes()
	if err == goredis.Nil {
		return nil, cacheStore.ErrNotFound
	}
	return value, err
}

// GetDel 使用 Redis GETDEL 原子读取并删除缓存值。
func (s *Store) GetDel(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.GetDel(ctx, s.prefix(key)).Bytes()
	if err == goredis.Nil {
		return nil, cacheStore.ErrNotFound
	}
	return value, err
}

// Set 写入缓存值。
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, s.prefix(key), value, ttl).Err()
}

// SetNX 使用 Redis SET NX 原子写入不存在的键。
func (s *Store) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.prefix(key), value, ttl).Result()
}

// Delete 删除缓存值。
func (s *Store) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.prefix(key)).Err()
}

// Has 判断缓存键是否存在。
func (s *Store) Has(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, s.prefix(key)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMulti 使用 MGET 按输入顺序批量获取缓存值。
func (s *Store) GetMulti(ctx context.Context, keys []string) ([][]byte, error) {
	if len(keys) == 0 {
		return [][]byte{}, nil
	}

	prefixed := make([]string, len(keys))
	for index, key := range keys {
		prefixed[index] = s.prefix(key)
	}

	results, err := s.client.MGet(ctx, prefixed...).Result()
	if err != nil {
		return nil, err
	}

	values := make([][]byte, len(keys))
	missing := false
	for index, result := range results {
		switch value := result.(type) {
		case string:
			values[index] = []byte(value)
		case []byte:
			values[index] = value
		case nil:
			missing = true
		default:
			missing = true
		}
	}
	if missing {
		return values, cacheStore.ErrNotFound
	}
	return values, nil
}

// SetMulti 使用 Pipeline 批量写入缓存值。
func (s *Store) SetMulti(ctx context.Context, items []cacheStore.Item) error {
	if len(items) == 0 {
		return nil
	}

	pipeline := s.client.Pipeline()
	for _, item := range items {
		pipeline.Set(ctx, s.prefix(item.Key), item.Value, item.TTL)
	}
	_, err := pipeline.Exec(ctx)
	return err
}

// Close 保留底层 Redis 客户端的调用方所有权。
func (s *Store) Close() error {
	return nil
}

// prefix 返回带命名空间前缀的缓存键。
func (s *Store) prefix(key string) string {
	return s.cfg.keyPrefix + key
}
