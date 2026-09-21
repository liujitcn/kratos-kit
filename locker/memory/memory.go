package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/liujitcn/kratos-kit/locker/contract"
)

// Locker 管理同一进程内按 key 隔离的互斥锁。
type Locker struct {
	mu    sync.Mutex
	locks map[string]*keyLock
}

type keyLock struct {
	sem  chan struct{}
	refs int
}

type lease struct {
	ctx       context.Context
	releaseFn func() error
	once      sync.Once
	err       error
}

// New 创建进程内锁。
func New() *Locker {
	return &Locker{locks: make(map[string]*keyLock)}
}

// Acquire 尝试取得指定 key 的进程内锁。
func (l *Locker) Acquire(ctx context.Context, key string, _ time.Duration) (contract.Lease, error) {
	if l == nil {
		return nil, errors.New("进程内锁未初始化")
	}
	if key == "" {
		return nil, errors.New("锁 key 不能为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	current, ok := l.locks[key]
	if !ok {
		current = &keyLock{sem: make(chan struct{}, 1)}
		l.locks[key] = current
	}
	current.refs++
	l.mu.Unlock()
	select {
	case current.sem <- struct{}{}:
		return &lease{ctx: ctx, releaseFn: func() error {
			<-current.sem
			l.mu.Lock()
			current.refs--
			if current.refs == 0 && l.locks[key] == current {
				delete(l.locks, key)
			}
			l.mu.Unlock()
			return nil
		}}, nil
	default:
		l.mu.Lock()
		current.refs--
		if current.refs == 0 && l.locks[key] == current {
			delete(l.locks, key)
		}
		l.mu.Unlock()
		return nil, contract.ErrNotObtained
	}
}

// Mode 返回进程内锁模式。
func (*Locker) Mode() contract.Mode { return contract.ModeMemory }

// Close 释放进程内锁管理器资源。
func (*Locker) Close() error { return nil }

// Context 返回租约绑定的上下文。
func (l *lease) Context() context.Context { return l.ctx }

// Release 释放锁租约。
func (l *lease) Release() error {
	l.once.Do(func() { l.err = l.releaseFn() })
	return l.err
}
