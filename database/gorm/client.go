package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"github.com/liujitcn/kratos-kit/database/gorm/logger"
	"github.com/liujitcn/kratos-kit/database/gorm/util"
	gormLog "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"gorm.io/plugin/opentelemetry/tracing"
	"gorm.io/plugin/prometheus"

	"gorm.io/gorm"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

// NewGormClient 创建GORM数据库客户端
func NewGormClient(cfg *conf.Data_Database) (*gorm.DB, error) {
	if cfg == nil {
		return nil, errors.New("gorm client config is nil")
	}
	log.Infof("Gorm SqlDb: %s => %s", util.Blue(cfg.Driver), util.Green(cfg.Source))
	// 获取驱动
	gormDriver, ok := driver.Opens[cfg.Driver]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Gorm驱动加载失败【%s】", cfg.Driver))
	}
	var lv gormLog.LogLevel
	if cfg.OpenLog {
		lv = gormLog.Info
	} else {
		lv = gormLog.Silent
	}

	db, err := gorm.Open(gormDriver(cfg.Source), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: logger.New(
			gormLog.Config{
				SlowThreshold: time.Second,
				Colorful:      true,
				LogLevel:      lv,
			},
		),
	})
	if err != nil {
		log.Fatalf("failed opening connection to db: %v", err)
	}

	if cfg.GetEnableTrace() {
		if err = db.Use(tracing.NewPlugin()); err != nil {
			log.Fatalf("failed enable trace: %v", err)
		}
	}

	if cfg.GetEnableMetrics() {
		if err = db.Use(prometheus.New(prometheus.Config{
			RefreshInterval: 15,                          // refresh metrics interval (default 15 seconds)
			StartServer:     true,                        // start http server to expose metrics
			DBName:          cfg.GetPrometheusDbName(),   // `DBName` as metrics label
			PushAddr:        cfg.GetPrometheusPushAddr(), // push metrics if `PushAddr` configured
			HTTPServerPort:  cfg.GetPrometheusHttpPort(), // configure http server port, default port 8080 (if you have configured multiple instances, only the first `HTTPServerPort` will be used to start server)
		})); err != nil {
			log.Fatalf("failed enable metrics: %v", err)
		}
	}

	var sqlDB *sql.DB
	sqlDB, err = db.DB()
	if sqlDB != nil {
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
	return db, nil
}
