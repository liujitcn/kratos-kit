package gorm

import "github.com/liujitcn/kratos-kit/database/gorm/internal/callback"

// RegisterMigrateModel 注册用于数据库迁移与回调字段识别的数据库模型。
func RegisterMigrateModel(model any) {
	callback.RegisterMigrateModel(model)
}

// RegisterMigrateModels 批量注册用于数据库迁移与回调字段识别的数据库模型。
func RegisterMigrateModels(models ...any) {
	callback.RegisterMigrateModels(models...)
}
