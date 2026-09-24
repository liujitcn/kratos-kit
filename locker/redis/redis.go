package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/locker/contract"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Redis 是基于 Redis 的分布式锁实现。
type Redis struct {
	client redis.UniversalClient
	mutex  *redislock.Client
}

type lease struct {
	ctx       context.Context
	releaseFn func() error
	once      sync.Once
	err       error
}

// New 创建 Redis 分布式锁实例。
func New(cfg *configv1.Data_Redis) (*Redis, error) {
	if cfg == nil {
		return nil, errors.New("Redis 配置不能为空")
	}
	if len(cfg.GetAddr()) == 0 || cfg.GetAddr()[0] == "" {
		return nil, errors.New("Redis 地址不能为空")
	}
	redisOptions, err := utils.GetUniversalOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("redis options failed: %w", err)
	}
	client := redis.NewUniversalClient(redisOptions)
	if client == nil {
		return nil, errors.New("failed opening connection to redis")
	}
	if cfg.GetEnableTracing() {
		if err = redisotel.InstrumentTracing(client); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed open tracing: %w", err)
		}
	}
	if cfg.GetEnableMetrics() {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed open metrics: %w", err)
		}
	}
	if _, err = client.Ping(context.Background()).Result(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed ping redis: %w", err)
	}
	return &Redis{client: client, mutex: redislock.New(client)}, nil
}

// Acquire 尝试取得指定 key 的 Redis 分布式锁并自动续租。
func (r *Redis) Acquire(ctx context.Context, key string, ttl time.Duration) (contract.Lease, error) {
	if r == nil || r.client == nil || r.mutex == nil {
		return nil, errors.New("Redis 分布式锁未初始化")
	}
	if key == "" {
		return nil, errors.New("锁 key 不能为空")
	}
	if ttl <= 0 {
		return nil, errors.New("锁 TTL 必须大于零")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lock, err := r.mutex.Obtain(ctx, key, ttl, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil, contract.ErrNotObtained
	}
	if err != nil {
		return nil, fmt.Errorf("获取 Redis 分布式锁失败: %w", err)
	}
	return newLease(ctx, lock, ttl), nil
}

// Mode 返回 Redis 分布式锁模式。
func (*Redis) Mode() contract.Mode { return contract.ModeRedis }

// Close 关闭 Redis 客户端。
func (r *Redis) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func newLease(parent context.Context, lock *redislock.Lock, ttl time.Duration) contract.Lease {
	leaseContext, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	refreshInterval := ttl / 3
	if refreshInterval < time.Second {
		refreshInterval = time.Second
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-leaseContext.Done():
				return
			case <-ticker.C:
				if err := lock.Refresh(context.Background(), ttl, nil); err != nil {
					log.Warn("Redis 分布式锁续租失败", "key", lock.Key(), "error", err)
					cancel()
					return
				}
			}
		}
	}()
	return &lease{ctx: leaseContext, releaseFn: func() error {
		cancel()
		<-done
		err := lock.Release(context.Background())
		if errors.Is(err, redislock.ErrLockNotHeld) {
			return nil
		}
		return err
	}}
}

// Context 返回租约绑定的上下文。
func (l *lease) Context() context.Context { return l.ctx }

// Release 释放锁租约。
func (l *lease) Release() error {
	l.once.Do(func() { l.err = l.releaseFn() })
	return l.err
}
