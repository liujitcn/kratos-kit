package local

import (
	"context"
	"sync"
	"time"

	"github.com/coocood/freecache"

	"github.com/liujitcn/kratos-kit/cache/store"
)

const (
	defaultSize = 256 * 1024 * 1024
	lockCount   = 256
)

// Option 配置本地缓存。
type Option func(*config)

type config struct {
	size       int
	defaultTTL time.Duration
}

// WithSize 设置预分配缓存大小，单位为字节。
func WithSize(size int) Option {
	return func(c *config) {
		if size > 0 {
			c.size = size
		}
	}
}

// WithDefaultTTL 设置调用方未指定 TTL 时使用的默认有效期。
func WithDefaultTTL(ttl time.Duration) Option {
	return func(c *config) {
		c.defaultTTL = ttl
	}
}

// New 创建本地缓存。
func New(opts ...Option) *Store {
	cfg := &config{size: defaultSize}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Store{
		cache: freecache.NewCache(cfg.size),
		cfg:   cfg,
	}
}

// Store 是基于 FreeCache 的缓存实现。
type Store struct {
	cache *freecache.Cache
	cfg   *config
	locks [lockCount]sync.Mutex
}

var _ store.Store = (*Store)(nil)

// Get 获取缓存值。
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	var value []byte
	value, err = s.cache.Get([]byte(key))
	if err == freecache.ErrNotFound {
		return nil, store.ErrNotFound
	}
	return value, err
}

// GetDel 原子读取并删除缓存值。
func (s *Store) GetDel(ctx context.Context, key string) ([]byte, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	lock := &s.locks[lockIndex(key)]
	lock.Lock()
	defer lock.Unlock()

	var value []byte
	value, err = s.cache.Get([]byte(key))
	if err == freecache.ErrNotFound {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.cache.Del([]byte(key))
	return value, nil
}

// Set 写入缓存值。
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := ctx.Err()
	if err != nil {
		return err
	}
	lock := &s.locks[lockIndex(key)]
	lock.Lock()
	defer lock.Unlock()
	return s.cache.Set([]byte(key), value, s.expireSeconds(ttl))
}

// SetNX 在键不存在时原子写入缓存值。
func (s *Store) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	err := ctx.Err()
	if err != nil {
		return false, err
	}

	lock := &s.locks[lockIndex(key)]
	lock.Lock()
	defer lock.Unlock()

	_, err = s.cache.Get([]byte(key))
	if err == nil {
		return false, nil
	}
	if err != freecache.ErrNotFound {
		return false, err
	}
	if err = s.cache.Set([]byte(key), value, s.expireSeconds(ttl)); err != nil {
		return false, err
	}
	return true, nil
}

// Delete 删除缓存值。
func (s *Store) Delete(ctx context.Context, key string) error {
	err := ctx.Err()
	if err != nil {
		return err
	}
	lock := &s.locks[lockIndex(key)]
	lock.Lock()
	defer lock.Unlock()
	s.cache.Del([]byte(key))
	return nil
}

// Has 判断缓存键是否存在。
func (s *Store) Has(ctx context.Context, key string) (bool, error) {
	err := ctx.Err()
	if err != nil {
		return false, err
	}

	_, err = s.cache.Get([]byte(key))
	if err == freecache.ErrNotFound {
		return false, nil
	}
	return err == nil, err
}

// GetMulti 按输入顺序批量获取缓存值。
func (s *Store) GetMulti(ctx context.Context, keys []string) ([][]byte, error) {
	values := make([][]byte, len(keys))
	missing := false
	var err error
	for index, key := range keys {
		err = ctx.Err()
		if err != nil {
			return nil, err
		}

		var value []byte
		value, err = s.cache.Get([]byte(key))
		if err == freecache.ErrNotFound {
			missing = true
			continue
		}
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	if missing {
		return values, store.ErrNotFound
	}
	return values, nil
}

// SetMulti 批量写入缓存值。
func (s *Store) SetMulti(ctx context.Context, items []store.Item) error {
	var err error
	for _, item := range items {
		err = ctx.Err()
		if err != nil {
			return err
		}
		lock := &s.locks[lockIndex(item.Key)]
		lock.Lock()
		err = s.cache.Set([]byte(item.Key), item.Value, s.expireSeconds(item.TTL))
		lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// Close 清空本地缓存。
func (s *Store) Close() error {
	s.cache.Clear()
	return nil
}

// EntryCount 返回当前缓存项数量。
func (s *Store) EntryCount() int64 {
	return s.cache.EntryCount()
}

// HitCount 返回缓存命中次数。
func (s *Store) HitCount() int64 {
	return s.cache.HitCount()
}

// MissCount 返回缓存未命中次数。
func (s *Store) MissCount() int64 {
	return s.cache.MissCount()
}

// EvacuateCount 返回被覆盖或淘汰的缓存项数量。
func (s *Store) EvacuateCount() int64 {
	return s.cache.EvacuateCount()
}

// ExpiredCount 返回已经过期的缓存项数量。
func (s *Store) ExpiredCount() int64 {
	return s.cache.ExpiredCount()
}

// expireSeconds 将 TTL 转为 FreeCache 使用的向上取整秒数。
func (s *Store) expireSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		ttl = s.cfg.defaultTTL
	}
	if ttl <= 0 {
		return 0
	}
	return int((ttl + time.Second - 1) / time.Second)
}

// lockIndex 返回缓存键对应的分片锁下标。
func lockIndex(key string) uint32 {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)

	hash := offset32
	for index := range len(key) {
		hash ^= uint32(key[index])
		hash *= prime32
	}
	return hash % lockCount
}
