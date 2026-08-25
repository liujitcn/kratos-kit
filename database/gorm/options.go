package gorm

import "slices"

// DefaultClientName 表示默认 GORM 客户端名称。
const DefaultClientName = "default"

// ClientOption 配置 GORM 客户端的名称和模型范围。
type ClientOption func(*clientOptions)

type clientOptions struct {
	// name 是客户端名称，供日志、指标和多数据源迁移目标识别使用。
	name string
	// modelsExplicit 表示是否显式指定了当前客户端的模型集合。
	modelsExplicit bool
	// migrateModels 是当前客户端参与自动建表和字段审计的模型集合。
	migrateModels []interface{}
}

// WithName 设置 GORM 客户端名称，用于日志、错误和指标标签。
func WithName(name string) ClientOption {
	return func(opts *clientOptions) {
		opts.name = name
	}
}

// WithMigrateModels 设置当前客户端专属的迁移和隔离模型。
func WithMigrateModels(models ...interface{}) ClientOption {
	return func(opts *clientOptions) {
		opts.modelsExplicit = true
		opts.migrateModels = slices.Clone(models)
	}
}
