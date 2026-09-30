package migration

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/liujitcn/kratos-kit/database/gorm"
)

// migrationLockTimeout 是获取迁移锁的最大等待时间。
const migrationLockTimeout = 30 * time.Second

// withMigrationLock 使用迁移记录数据库锁定一个模块的数据源迁移。
func withMigrationLock(
	ctx context.Context,
	client *gorm.Client,
	module ModuleName,
	dataSource string,
	fn func() error,
) error {
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
	lockName := "kratos_migration_" + module.String() + "_" + dataSource
	if len(lockName) > 64 {
		lockName = lockName[:64]
	}
	switch client.Driver() {
	case databaseTypePostgres:
		err = lockWithPostgres(ctx, conn, lockName)
	case databaseTypeMySQL, databaseTypeDoris:
		err = lockWithMySQL(ctx, conn, lockName)
	default:
		err = fmt.Errorf("迁移记录数据库驱动 %s 不支持迁移锁", client.Driver())
	}
	if err != nil {
		return err
	}
	defer func() {
		unlockMigration(client.Driver(), conn, lockName)
	}()
	return fn()
}

// lockWithMySQL 通过 MySQL GET_LOCK 获取迁移锁，超时返回失败。
func lockWithMySQL(ctx context.Context, conn *sql.Conn, lockName string) error {
	var acquired int
	err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 30)", lockName).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("获取迁移锁超时: %s", lockName)
	}
	return nil
}

// lockWithPostgres 通过 PostgreSQL 会话级咨询锁获取迁移锁，在超时窗口内轮询重试。
func lockWithPostgres(ctx context.Context, conn *sql.Conn, lockName string) error {
	var key int64
	var acquired bool
	var err error
	key, err = advisoryLockKey(lockName)
	if err != nil {
		return fmt.Errorf("计算迁移锁键失败: %w", err)
	}
	deadline := time.Now().Add(migrationLockTimeout)
	for {
		err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired)
		if err != nil {
			return fmt.Errorf("获取迁移锁失败: %w", err)
		}
		if acquired {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("获取迁移锁超时: %s", lockName)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// unlockMigration 释放迁移锁，释放失败不影响迁移结果。
func unlockMigration(driverName string, conn *sql.Conn, lockName string) {
	switch driverName {
	case databaseTypePostgres:
		var key int64
		var err error
		key, err = advisoryLockKey(lockName)
		if err != nil {
			return
		}
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
	case databaseTypeMySQL, databaseTypeDoris:
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
	}
}

// advisoryLockKey 把锁名哈希为 PostgreSQL 咨询锁的 64 位整型键。
func advisoryLockKey(lockName string) (int64, error) {
	var err error
	hasher := fnv.New64a()
	_, err = hasher.Write([]byte(lockName))
	if err != nil {
		return 0, err
	}
	return int64(hasher.Sum64()), nil
}
