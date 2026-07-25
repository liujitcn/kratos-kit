package migration

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
)

// DefaultTarget 表示默认业务数据库目标。
const DefaultTarget = databaseGorm.DefaultClientName

// Migration 描述一个模块提供的版本化迁移资源。
type Migration struct {
	// FS 表示迁移脚本所在的嵌入文件系统。
	FS fs.FS
	// Path 表示迁移脚本在文件系统中的目录。
	Path string
	// Dependencies 表示当前迁移模块依赖的其他模块。
	Dependencies []string
}

// Contributor 表示可向应用贡献数据库迁移资源的模块。
type Contributor interface {
	// Name 返回迁移模块注册名称。
	Name() string
	// Migrations 返回迁移模块提供的版本化资源。
	Migrations() []Migration
}

// AdditionalMigrations 表示由接入项目贡献的迁移模块集合。
type AdditionalMigrations = []Contributor

// Runner 执行已注册模块的数据库迁移。
type Runner struct {
	// registry 保存已校验的迁移模块及其依赖关系。
	registry *Registry
	// clients 按数据源名称保存已注入的 GORM 客户端。
	clients map[string]*databaseGorm.Client
	// mu 保护客户端注册和迁移执行过程，避免并发执行同一 Runner。
	mu sync.Mutex
}

// NewRunner 创建迁移执行器。
func NewRunner(registry *Registry) (*Runner, error) {
	if registry == nil {
		return nil, fmt.Errorf("迁移注册表不能为空")
	}
	return &Runner{
		registry: registry,
		clients:  make(map[string]*databaseGorm.Client),
	}, nil
}

// SetClient 注入数据库客户端；默认客户端用于保存集中迁移记录。
func (r *Runner) SetClient(client *databaseGorm.Client) error {
	if r == nil {
		return fmt.Errorf("迁移执行器不能为空")
	}
	if client == nil || client.DB == nil {
		return fmt.Errorf("迁移目标数据库客户端不能为空")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[client.Name()] = client
	return nil
}

// NewReady 创建默认数据库迁移屏障。
func NewReady(_ *databaseGorm.Client) Ready {
	return Ready{}
}

// Run 执行指定迁移模块及其依赖模块。
func (r *Runner) Run(ctx context.Context, name string, targetClient *databaseGorm.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	centralClient, exists := r.clients[DefaultTarget]
	if !exists || centralClient == nil || centralClient.DB == nil {
		return fmt.Errorf("迁移记录数据库客户端不能为空")
	}
	if !centralClient.MigrationEnabled() {
		return nil
	}
	if targetClient == nil || targetClient.DB == nil {
		return fmt.Errorf("迁移目标数据库客户端不能为空")
	}
	if !targetClient.MigrationEnabled() {
		return nil
	}
	if _, exists := r.registry.migrations[name]; !exists {
		return fmt.Errorf("迁移模块未注册: %s", name)
	}
	visited := make(map[string]bool)
	return r.runModule(ctx, centralClient, name, targetClient, visited)
}

// runModule 按依赖顺序执行迁移模块。
func (r *Runner) runModule(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	name string,
	targetClient *databaseGorm.Client,
	visited map[string]bool,
) error {
	if visited[name] {
		return nil
	}
	migrations := r.registry.migrations[name]
	if len(migrations) == 0 {
		return fmt.Errorf("迁移模块未提供资源: %s", name)
	}
	var err error
	for _, dependency := range migrations[0].Dependencies {
		err = r.runModule(ctx, centralClient, dependency, centralClient, visited)
		if err != nil {
			return err
		}
	}
	for _, migration := range migrations {
		err = r.runMigration(ctx, centralClient, targetClient, name, migration)
		if err != nil {
			return fmt.Errorf("执行迁移模块 %s 数据源 %s 失败: %w", name, targetClient.Name(), err)
		}
	}
	visited[name] = true
	return nil
}

// runMigration 执行单个迁移资源目录，并将执行记录保存到默认数据源。
func (r *Runner) runMigration(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	targetClient *databaseGorm.Client,
	moduleName string,
	migration Migration,
) error {
	if targetClient == nil || targetClient.DB == nil {
		return fmt.Errorf("迁移模块 %s 数据库客户端不能为空", moduleName)
	}
	if !targetClient.MigrationEnabled() {
		return nil
	}
	centralDriver := centralClient.Dialector.Name()
	if centralDriver != "mysql" {
		return fmt.Errorf("迁移记录数据源暂不支持数据库驱动 %s", centralDriver)
	}
	targetDriver := targetClient.Dialector.Name()
	if targetDriver != "mysql" {
		return fmt.Errorf("迁移模块 %s 暂不支持数据库驱动 %s", moduleName, targetDriver)
	}
	assets, err := loadMigrationAssets(migration.FS, migration.Path)
	if err != nil {
		return err
	}
	business := targetClient.Name()
	err = withMigrationLock(ctx, centralClient, business, func() error {
		return applyMigrationAssets(ctx, centralClient, targetClient, business, assets)
	})
	if err != nil {
		return fmt.Errorf("执行迁移模块 %s 失败: %w", moduleName, err)
	}
	return nil
}

// Ready 表示默认数据库已完成迁移，可供业务模块建立依赖。
type Ready struct{}
