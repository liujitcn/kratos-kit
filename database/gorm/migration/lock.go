package migration

import (
	"context"
	"database/sql"
	"fmt"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
)

// withMigrationLock 使用默认数据库锁定一个迁移业务。
func withMigrationLock(ctx context.Context, client *databaseGorm.Client, business string, fn func() error) error {
	var sqlDB *sql.DB
	var err error
	sqlDB, err = client.DB.DB()
	if err != nil {
		return fmt.Errorf("获取迁移记录数据库连接失败: %w", err)
	}
	var conn *sql.Conn
	conn, err = sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移记录数据库连接失败: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	lockName := "kratos_migration_" + business
	if len(lockName) > 64 {
		lockName = lockName[:64]
	}
	var acquired int
	err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 30)", lockName).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("迁移业务 %s 获取锁超时", business)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
	}()
	return fn()
}
