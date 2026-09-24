package locker

import (
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/locker/contract"
	"github.com/liujitcn/kratos-kit/locker/memory"
	"github.com/liujitcn/kratos-kit/locker/redis"
)

const (
	// ModeMemory 表示进程内锁模式。
	ModeMemory = contract.ModeMemory
	// ModeRedis 表示 Redis 分布式锁模式。
	ModeRedis = contract.ModeRedis
)

var (
	// ErrNotObtained 表示指定锁当前已被持有。
	ErrNotObtained = contract.ErrNotObtained
)

// Mode 表示锁的运行模式。
type Mode = contract.Mode

// Lease 表示已经取得且可释放的锁租约。
type Lease = contract.Lease

// Locker 定义统一的内存锁和分布式锁能力。
type Locker = contract.Locker

// NewLocker 根据 Redis 配置创建锁；未配置 Redis 时使用进程内锁。
func NewLocker(cfg *configv1.Data_Redis) (Locker, error) {
	if cfg == nil {
		return memory.New(), nil
	}
	return redis.New(cfg)
}
