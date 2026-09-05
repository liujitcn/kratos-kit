package gorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"gorm.io/gorm"
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
	// DB 是实际执行查询、事务和迁移脚本的 GORM 数据库对象。
	*gorm.DB
	// name 是数据源名称，default 表示集中保存迁移记录的默认库。
	name string
	// driver 是配置声明的真实数据库驱动，用于区分复用同一 GORM Dialector 的数据库。
	driver string
	// migrateEnabled 表示当前数据源是否允许执行 GORM 自动迁移。
	migrateEnabled bool
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
		clientLabel = DefaultClientName
	}
	log.Info(fmt.Sprintf("GORM SQL DB[%s]: %s", clientLabel, util.Blue(cfg.Driver)))

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

	source := cfg.Source
	if cfg.Driver == "mysql" || cfg.Driver == "doris" {
		source = ensureMySQLMultiStatements(source)
	}
	db, err := gorm.Open(gormDriver(source), &gorm.Config{
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

	registry := newMigrateRegistry(clientOpts.migrateModels, clientOpts.modelsExplicit)
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

	client := &Client{
		DB:             db,
		name:           clientLabel,
		driver:         cfg.Driver,
		migrateEnabled: cfg.GetEnableMigrate(),
	}

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

// MigrationEnabled 返回当前客户端是否允许执行 GORM 自动迁移。
func (c *Client) MigrationEnabled() bool {
	return c != nil && c.migrateEnabled
}

// Name 返回当前客户端的数据源名称。
func (c *Client) Name() string {
	if c == nil || c.name == "" {
		return DefaultClientName
	}
	return c.name
}

// Driver 返回配置声明的真实数据库驱动。
func (c *Client) Driver() string {
	if c == nil {
		return ""
	}
	if c.driver != "" {
		return c.driver
	}
	if c.DB != nil && c.Dialector != nil {
		return c.Dialector.Name()
	}
	return ""
}

// registerCallbacks 按注册顺序将包级回调安装到 GORM 客户端。
func registerCallbacks(db *gorm.DB) error {
	var err error
	for i, fn := range getCallbackQueries() {
		err = db.Callback().Query().Before("gorm:query").Register(fmt.Sprintf("before_query_%d", i), fn)
		if err != nil {
			return err
		}
	}
	for i, fn := range getCallbackQueryAfters() {
		err = db.Callback().Query().After("gorm:after_query").Register(fmt.Sprintf("after_query_%d", i), fn)
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
		err = db.Callback().Raw().Before("gorm:raw").Register(fmt.Sprintf("before_raw_%d", i), fn) //nolint:forbidigo
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
	for i, fn := range getCallbackCreateAfters() {
		err = db.Callback().Create().After("gorm:after_create").Before("gorm:commit_or_rollback_transaction").Register(fmt.Sprintf("after_create_%d", i), fn)
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
	for i, fn := range getCallbackUpdateAfters() {
		err = db.Callback().Update().After("gorm:after_update").Before("gorm:commit_or_rollback_transaction").Register(fmt.Sprintf("after_update_%d", i), fn)
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
	for i, fn := range getCallbackDeleteAfters() {
		err = db.Callback().Delete().After("gorm:after_delete").Before("gorm:commit_or_rollback_transaction").Register(fmt.Sprintf("after_delete_%d", i), fn)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureMySQLMultiStatements 为支持版本化 SQL 迁移的 MySQL 连接补充多语句执行参数。
func ensureMySQLMultiStatements(source string) string {
	lowerSource := strings.ToLower(source)
	const disabledMultiStatements = "multistatements=false"
	if index := strings.Index(lowerSource, disabledMultiStatements); index >= 0 {
		return source[:index] + "multiStatements=true" + source[index+len(disabledMultiStatements):]
	}
	if strings.Contains(lowerSource, "multistatements=") {
		return source
	}
	if strings.Contains(source, "?") {
		return source + "&multiStatements=true"
	}
	return source + "?multiStatements=true"
}
