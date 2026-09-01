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

// Entry 表示一个缓存条目的只读快照。
type Entry struct {
	// Key 是缓存键。
	Key string
	// Type 是缓存类型，支持 string 和 hash。
	Type string
	// Value 是字符串缓存值。
	Value string
	// Fields 是 Hash 缓存字段。
	Fields map[string]string
	// TTL 是剩余有效期，-1 表示永不过期。
	TTL time.Duration
	// ExpiresAt 是过期时间，永不过期时为空。
	ExpiresAt time.Time
	// CreatedAt 是首次写入或首次观测时间。
	CreatedAt time.Time
	// UpdatedAt 是最近写入或过期时间变更时间。
	UpdatedAt time.Time
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
