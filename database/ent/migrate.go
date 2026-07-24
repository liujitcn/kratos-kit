package ent

import (
	"context"
	"sync"

	"entgo.io/ent/dialect"
)

// MigrateFunc 定义 Ent 自动迁移函数。
type MigrateFunc func(context.Context, dialect.Driver) error

var (
	registeredMigrationsMu sync.RWMutex
	registeredMigrations   []MigrateFunc
)

// RegisterMigrate 注册用于数据库迁移的 Ent 迁移函数。
func RegisterMigrate(fn MigrateFunc) {
	if fn == nil {
		return
	}
	registeredMigrationsMu.Lock()
	defer registeredMigrationsMu.Unlock()
	registeredMigrations = append(registeredMigrations, fn)
}

// RegisterMigrates 注册多个用于数据库迁移的 Ent 迁移函数。
func RegisterMigrates(fn ...MigrateFunc) {
	if len(fn) == 0 {
		return
	}
	registeredMigrationsMu.Lock()
	defer registeredMigrationsMu.Unlock()
	for _, migrate := range fn {
		if migrate == nil {
			continue
		}
		registeredMigrations = append(registeredMigrations, migrate)
	}
}

// getRegisteredMigrations 返回已注册的迁移函数副本。
func getRegisteredMigrations() []MigrateFunc {
	registeredMigrationsMu.RLock()
	defer registeredMigrationsMu.RUnlock()
	if len(registeredMigrations) == 0 {
		return nil
	}
	dup := make([]MigrateFunc, len(registeredMigrations))
	copy(dup, registeredMigrations)
	return dup
}

// hasRegisteredMigrations 判断当前是否已注册迁移函数。
func hasRegisteredMigrations() bool {
	registeredMigrationsMu.RLock()
	defer registeredMigrationsMu.RUnlock()
	return len(registeredMigrations) > 0
}
