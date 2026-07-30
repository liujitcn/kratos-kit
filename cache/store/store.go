// Package store 定义支持 context、原子消费、原子写入和批量操作的缓存契约。
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 表示缓存键不存在或已经过期。
var ErrNotFound = errors.New("cache: key not found")

// Item 表示一个批量写入项。
type Item struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

// Store 定义面向字节值的现代缓存接口。
type Store interface {
	Get(context.Context, string) ([]byte, error)
	// GetDel 原子读取并删除缓存值。
	GetDel(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	SetNX(context.Context, string, []byte, time.Duration) (bool, error)
	Delete(context.Context, string) error
	Has(context.Context, string) (bool, error)
	GetMulti(context.Context, []string) ([][]byte, error)
	SetMulti(context.Context, []Item) error
	Close() error
}
