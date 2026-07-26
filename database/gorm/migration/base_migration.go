package migration

import (
	"time"
)

// baseMigration 数据库迁移记录。
type baseMigration struct {
	ID          int64     `gorm:"column:id;type:bigint;primaryKey;autoIncrement:true;comment:主键ID" json:"id"`                                               // 主键ID
	Module      string    `gorm:"column:module;type:varchar(64);not null;uniqueIndex:unique_base_migration,priority:1;comment:迁移模块" json:"module"`          // 迁移模块
	DataSource  string    `gorm:"column:data_source;type:varchar(20);not null;uniqueIndex:unique_base_migration,priority:2;comment:数据源" json:"data_source"` // 数据源
	Version     string    `gorm:"column:version;type:varchar(50);not null;uniqueIndex:unique_base_migration,priority:3;comment:迁移版本" json:"version"`        // 迁移版本
	UpSql       string    `gorm:"column:up_sql;type:longtext;comment:升级脚本" json:"up_sql"`                                                                   // 升级脚本
	DownSql     string    `gorm:"column:down_sql;type:longtext;comment:回退脚本" json:"down_sql"`                                                               // 回退脚本
	Description string    `gorm:"column:description;type:longtext;comment:升级描述" json:"description"`                                                         // 升级描述
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`                                                           // 创建时间
}

// TableName 返回数据库迁移记录表名。
func (*baseMigration) TableName() string {
	return "base_migration"
}
