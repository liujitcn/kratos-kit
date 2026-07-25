package migration

import (
	"time"
)

// BaseMigration 数据库迁移记录
type baseMigration struct {
	ID          int64     `gorm:"column:id;type:bigint;primaryKey;comment:主键ID" json:"id"`          // 主键ID
	Business    string    `gorm:"column:business;type:varchar(20);comment:迁移业务" json:"business"`    // 迁移业务
	Version     string    `gorm:"column:version;type:varchar(50);comment:迁移版本" json:"version"`      // 迁移版本
	Description string    `gorm:"column:description;type:text;comment:升级描述" json:"description"`     // 升级描述
	IsSuccess   bool      `gorm:"column:is_success;type:tinyint(1);comment:是否成功" json:"is_success"` // 是否成功
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`   // 创建时间
}

// TableName baseMigration's table name
func (*baseMigration) TableName() string {
	return "base_migration"
}
