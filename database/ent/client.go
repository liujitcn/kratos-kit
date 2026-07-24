package ent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect"
	dialectSql "entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/database/ent/driver"
	"github.com/liujitcn/kratos-kit/database/ent/util"
)

// Client 封装 Ent 底层 dialect.Driver 与 database/sql 连接池。
type Client struct {
	dialect.Driver
	sqlDB       *sql.DB
	dialectName string
}

// DB 返回底层 database/sql 连接池。
func (c *Client) DB() *sql.DB {
	return c.sqlDB
}

// Close 关闭 Ent 底层数据库连接。
func (c *Client) Close() error {
	if c == nil || c.sqlDB == nil {
		return nil
	}
	return c.sqlDB.Close()
}

// NewEntClient 创建 Ent 数据库客户端。
func NewEntClient(cfg *configv1.Data_Database) (*Client, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("ent client config is nil")
	}
	log.Info(fmt.Sprintf("Ent SqlDb: %s => %s", util.Blue(cfg.Driver), util.Green(cfg.Source)))

	open, ok := driver.Opens[cfg.Driver]
	if !ok {
		return nil, nil, fmt.Errorf("Ent驱动加载失败【%s】", cfg.Driver)
	}

	sqlDB, entDialect, err := open(cfg.Source)
	if err != nil {
		return nil, nil, fmt.Errorf("failed opening connection to db: %w", err)
	}
	configurePool(sqlDB, cfg)

	cleanUp := func() {
		if sqlDB == nil {
			return
		}
		closeErr := sqlDB.Close()
		if closeErr != nil {
			log.Error("failed close sql db", "error", closeErr)
			return
		}
	}

	drv := dialect.Driver(dialectSql.OpenDB(entDialect, sqlDB))
	drv, err = wrapTelemetry(drv, entDialect, cfg.GetEnableTrace(), cfg.GetEnableMetrics())
	if err != nil {
		cleanUp()
		return nil, cleanUp, err
	}
	if cfg.Debug {
		drv = dialect.DebugWithContext(drv, logDebugSQL)
	}

	client := &Client{
		Driver:      drv,
		sqlDB:       sqlDB,
		dialectName: entDialect,
	}

	if cfg.EnableMigrate && hasRegisteredMigrations() {
		err = client.RunRegisteredMigrations(context.Background())
		if err != nil {
			cleanUp()
			return nil, cleanUp, err
		}
		err = client.RunRegisteredTableComments(context.Background())
		if err != nil {
			cleanUp()
			return nil, cleanUp, err
		}
	}

	return client, cleanUp, nil
}

// RunRegisteredMigrations 执行已注册的 Ent 迁移函数。
func (c *Client) RunRegisteredMigrations(ctx context.Context) error {
	migrations := getRegisteredMigrations()
	for _, migrate := range migrations {
		err := migrate(ctx, c.Driver)
		if err != nil {
			return err
		}
	}
	return nil
}

// configurePool 按配置更新 database/sql 连接池参数。
func configurePool(sqlDB *sql.DB, cfg *configv1.Data_Database) {
	if cfg.MaxIdleConnections != nil {
		sqlDB.SetMaxIdleConns(int(cfg.GetMaxIdleConnections()))
	}
	if cfg.MaxOpenConnections != nil {
		sqlDB.SetMaxOpenConns(int(cfg.GetMaxOpenConnections()))
	}
	if cfg.ConnectionMaxLifetime != nil {
		sqlDB.SetConnMaxLifetime(cfg.GetConnectionMaxLifetime().AsDuration())
	}
}

// logDebugSQL 输出 Ent debug SQL 日志。
func logDebugSQL(_ context.Context, args ...any) {
	log.Debug(fmt.Sprint(args...))
}

var _ dialect.Driver = (*Client)(nil)
var _ interface {
	DB() *sql.DB
	Close() error
} = (*Client)(nil)
