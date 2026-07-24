package gorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	gormdb "gorm.io/gorm"
	gormLog "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/opentelemetry/tracing"
	"gorm.io/plugin/prometheus"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"github.com/liujitcn/kratos-kit/database/gorm/logger"
	"github.com/liujitcn/kratos-kit/database/gorm/util"
)

// Client 封装 GORM 数据库客户端。
type Client struct {
	*gormdb.DB
	name string
}

const defaultMetricsHTTPPort uint32 = 8080

var reservedMetricsPorts sync.Map

// NewGormClient 创建 GORM 数据库客户端。
func NewGormClient(cfg *configv1.Data_Database, options ...ClientOption) (*Client, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("gorm client config is nil")
	}
	var clientOpts clientOptions
	for _, option := range options {
		if option != nil {
			option(&clientOpts)
		}
	}
	clientLabel := clientOpts.name
	if clientLabel == "" {
		clientLabel = "default"
	}
	log.Info(fmt.Sprintf("GORM SQL DB[%s]: %s => %s", clientLabel, util.Blue(cfg.Driver), util.Green(cfg.Source)))

	gormDriver, driverExists := driver.Opens[cfg.Driver]
	if !driverExists {
		return nil, nil, fmt.Errorf("GORM 驱动加载失败【%s】", cfg.Driver)
	}
	var logLevel gormLog.LogLevel
	if cfg.Debug {
		logLevel = gormLog.Info
	} else {
		logLevel = gormLog.Silent
	}

	db, err := gormdb.Open(gormDriver(cfg.Source), &gormdb.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: logger.New(
			gormLog.Config{
				SlowThreshold: time.Second,
				Colorful:      true,
				LogLevel:      logLevel,
			},
		),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed opening connection to db[%s]: %w", clientLabel, err)
	}

	if cfg.GetEnableTrace() {
		if err = db.Use(tracing.NewPlugin()); err != nil {
			return nil, nil, fmt.Errorf("failed enable trace[%s]: %w", clientLabel, err)
		}
	}

	var metricsPort uint32
	if cfg.GetEnableMetrics() {
		metricsPort = cfg.GetPrometheusHttpPort()
		if metricsPort == 0 {
			metricsPort = defaultMetricsHTTPPort
		}
		if _, loaded := reservedMetricsPorts.LoadOrStore(metricsPort, clientLabel); loaded {
			return nil, nil, fmt.Errorf("GORM 指标端口冲突【%d】，数据源=%s", metricsPort, clientLabel)
		}
		metricsDBName := cfg.GetPrometheusDbName()
		if metricsDBName == "" {
			metricsDBName = clientLabel
		}
		if err = db.Use(prometheus.New(prometheus.Config{
			RefreshInterval: 15,                          // 指标刷新间隔，默认 15 秒。
			StartServer:     true,                        // 启动指标 HTTP 服务。
			DBName:          metricsDBName,               // Prometheus 数据库标签。
			PushAddr:        cfg.GetPrometheusPushAddr(), // Prometheus Pushgateway 地址。
			HTTPServerPort:  metricsPort,                 // 指标 HTTP 服务端口，默认 8080。
		})); err != nil {
			reservedMetricsPorts.Delete(metricsPort)
			return nil, nil, fmt.Errorf("failed enable metrics[%s]: %w", clientLabel, err)
		}
	}

	var sqlDB *sql.DB
	sqlDB, err = db.DB()
	if err != nil {
		if metricsPort != 0 {
			reservedMetricsPorts.Delete(metricsPort)
		}
		return nil, nil, fmt.Errorf("get sql db[%s]: %w", clientLabel, err)
	}

	cleanup := func() {
		closeErr := sqlDB.Close()
		if closeErr != nil {
			log.Error("failed close sql db", "error", closeErr)
		}
	}

	connectionTimeout := 5 * time.Second
	if cfg.GetConnectionTimeout() != nil {
		connectionTimeout = cfg.GetConnectionTimeout().AsDuration()
		if connectionTimeout <= 0 {
			return nil, cleanup, errors.New("gorm connection timeout must be greater than zero")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	err = sqlDB.PingContext(ctx)
	cancel()
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed ping database[%s]: %w", clientLabel, err)
	}

	registry := newMigrateRegistry(clientOpts.migrateModels, clientOpts.migrateSet)
	db = db.Set(migrateRegistryKey, registry)
	if err = registerCallbacks(db); err != nil {
		return nil, cleanup, err
	}
	if cfg.MaxIdleConnections != nil {
		sqlDB.SetMaxIdleConns(int(cfg.GetMaxIdleConnections()))
	}
	if cfg.MaxOpenConnections != nil {
		sqlDB.SetMaxOpenConns(int(cfg.GetMaxOpenConnections()))
	}
	if cfg.ConnectionMaxLifetime != nil {
		sqlDB.SetConnMaxLifetime(cfg.GetConnectionMaxLifetime().AsDuration())
	}

	client := &Client{DB: db, name: clientOpts.name}

	// 自动迁移会汇总当前客户端模型，并补充数据库表注释。
	if cfg.EnableMigrate {
		models := getMigrateModels(db)
		if len(models) > 0 {
			// 自动迁移和注释回填属于可信系统任务，允许执行原生 SQL。
			systemDB := SkipDataIsolation(client.DB)
			if err = systemDB.AutoMigrate(models...); err != nil {
				return nil, cleanup, err
			}
			if err = applyRegisteredTableComments(systemDB, models...); err != nil {
				return nil, cleanup, err
			}
		}
	}

	return client, cleanup, nil
}

// registerCallbacks 按注册顺序将包级回调安装到 GORM 客户端。
func registerCallbacks(db *gormdb.DB) error {
	var err error
	for i, fn := range getCallbackQueries() {
		err = db.Callback().Query().Before("gorm:query").Register(fmt.Sprintf("before_query_%d", i), fn)
		if err != nil {
			return err
		}
	}
	for i, fn := range getCallbackRows() {
		err = db.Callback().Row().Before("gorm:row").Register(fmt.Sprintf("before_row_%d", i), fn)
		if err != nil {
			return err
		}
	}
	for i, fn := range getCallbackRaws() {
		err = db.Callback().Raw().Before("gorm:raw").Register(fmt.Sprintf("before_raw_%d", i), fn)
		if err != nil {
			return err
		}
	}
	for i, fn := range getCallbackCreates() {
		err = db.Callback().Create().Before("gorm:before_create").Register(fmt.Sprintf("before_create_%d", i), fn)
		if err != nil {
			return err
		}
	}
	for i, item := range getCallbackUpdates() {
		err = db.Callback().Update().Before(item.anchor).Register(fmt.Sprintf("before_update_%d", i), item.fn)
		if err != nil {
			return err
		}
	}
	for i, fn := range getCallbackDeletes() {
		err = db.Callback().Delete().Before("gorm:delete").Register(fmt.Sprintf("before_delete_%d", i), fn)
		if err != nil {
			return err
		}
	}
	return nil
}
