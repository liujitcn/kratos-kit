package gorm

// DefaultClientName 表示默认 GORM 客户端名称。
const DefaultClientName = "default"

// ClientOption 配置 GORM 客户端的名称和模型范围。
type ClientOption func(*clientOptions)

type clientOptions struct {
	name          string
	migrateSet    bool
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
		opts.migrateSet = true
		opts.migrateModels = append([]interface{}(nil), models...)
	}
}
