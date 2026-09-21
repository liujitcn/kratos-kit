package contract

import (
	"context"
	"errors"
	"time"
)

const (
	// ModeMemory 表示进程内锁模式。
	ModeMemory Mode = "memory"
	// ModeRedis 表示 Redis 分布式锁模式。
	ModeRedis Mode = "redis"
)

var (
	// ErrNotObtained 表示指定锁当前已被持有。
	ErrNotObtained = errors.New("锁未获取")
)

// Mode 表示锁的运行模式。
type Mode string

// Lease 表示已经取得且可释放的锁租约。
type Lease interface {
	Context() context.Context
	Release() error
}

// Locker 定义统一的内存锁和分布式锁能力。
type Locker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (Lease, error)
	Mode() Mode
	Close() error
}
