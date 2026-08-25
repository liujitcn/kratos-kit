package migration

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"sync"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
)

// DefaultTarget 表示默认数据源目标。
const DefaultTarget = databaseGorm.DefaultClientName

// Migration 描述一个模块提供的版本化迁移资源。
type Migration struct {
	// FS 表示迁移脚本所在的嵌入文件系统。
	FS fs.FS
	// Path 表示迁移版本目录所在的资源根目录。
	Path string
	// Dependencies 表示当前迁移模块依赖的其他模块。
	Dependencies []ModuleName
}

// Contributor 表示可向应用贡献数据库迁移资源的模块。
type Contributor interface {
	// Name 返回迁移模块注册名称。
	Name() ModuleName
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
//
// 不传目标客户端时，按版本目录中的数据源目录查找客户端，未找到时回退到默认客户端；
// 传入一个目标客户端时，仅执行该客户端对应数据库类型和数据源的迁移。
func (r *Runner) Run(ctx context.Context, name ModuleName, targetClients ...*databaseGorm.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	centralClient, exists := r.clients[DefaultTarget]
	if !exists || centralClient == nil || centralClient.DB == nil {
		return fmt.Errorf("迁移记录数据库客户端不能为空")
	}
	_, exists = r.registry.migrations[name]
	if !exists {
		return fmt.Errorf("迁移模块未注册: %s", name)
	}
	if len(targetClients) > 1 {
		return fmt.Errorf("迁移目标数据库客户端最多只能传入一个")
	}
	assetsByModule := make(map[ModuleName][]migrationAsset)
	err := r.loadMigrationTree(name, assetsByModule, make(map[ModuleName]bool))
	if err != nil {
		return err
	}
	targets := make(map[string]*databaseGorm.Client)
	if len(targetClients) == 1 {
		targetClient := targetClients[0]
		if targetClient == nil || targetClient.DB == nil {
			return fmt.Errorf("迁移目标数据库客户端不能为空")
		}
		targets[targetClient.Name()] = targetClient
	} else {
		for _, assets := range assetsByModule {
			for _, asset := range assets {
				var targetClient *databaseGorm.Client
				targetClient, exists = r.clients[asset.dataSource]
				if !exists || targetClient == nil || targetClient.DB == nil {
					// 迁移目录可以提前声明尚未配置的数据源，未匹配时统一落到默认数据源。
					targetClient, exists = r.clients[DefaultTarget]
				}
				if !exists || targetClient == nil || targetClient.DB == nil {
					return fmt.Errorf("迁移目标数据源客户端未注入: %s", asset.dataSource)
				}
				targets[asset.dataSource] = targetClient
			}
		}
	}
	targetNames := make([]string, 0, len(targets))
	for targetName := range targets {
		targetNames = append(targetNames, targetName)
	}
	slices.Sort(targetNames)
	for _, targetName := range targetNames {
		targetClient := targets[targetName]
		targetDriver := targetClient.Driver()
		if targetDriver != databaseTypeMySQL && targetDriver != databaseTypeDoris {
			return fmt.Errorf("迁移模块 %s 暂不支持数据库驱动 %s", name, targetDriver)
		}
		hasMatchingAsset := false
		for _, assets := range assetsByModule {
			for _, asset := range assets {
				if asset.dataSource == targetName && asset.databaseType == targetDriver {
					hasMatchingAsset = true
					break
				}
			}
			if hasMatchingAsset {
				break
			}
		}
		if !hasMatchingAsset {
			return fmt.Errorf("迁移模块 %s 及其依赖的数据源 %s 未提供数据库类型 %s 的迁移资源", name, targetName, targetDriver)
		}
		visited := make(map[ModuleName]bool)
		err = r.runModule(ctx, centralClient, name, targetName, targetClient, visited, assetsByModule)
		if err != nil {
			return fmt.Errorf("执行迁移模块 %s 数据源 %s 失败: %w", name, targetName, err)
		}
	}
	return nil
}

// loadMigrationTree 加载当前模块及其依赖模块的迁移资源。
func (r *Runner) loadMigrationTree(
	name ModuleName,
	assetsByModule map[ModuleName][]migrationAsset,
	visited map[ModuleName]bool,
) error {
	if visited[name] {
		return nil
	}
	visited[name] = true
	migrations := r.registry.migrations[name]
	if len(migrations) == 0 {
		return fmt.Errorf("迁移模块未提供资源: %s", name)
	}
	var err error
	_, err = r.loadModuleAssets(name, migrations, assetsByModule)
	if err != nil {
		return err
	}
	for _, dependency := range migrationDependencies(migrations) {
		if err = r.loadMigrationTree(dependency, assetsByModule, visited); err != nil {
			return err
		}
	}
	return nil
}

// loadModuleAssets 加载一个迁移模块的全部数据源资源。
func (r *Runner) loadModuleAssets(
	name ModuleName,
	migrations []Migration,
	assetsByModule map[ModuleName][]migrationAsset,
) ([]migrationAsset, error) {
	if assets, exists := assetsByModule[name]; exists {
		return assets, nil
	}
	assets := make([]migrationAsset, 0)
	assetKeys := make(map[string]bool)
	for _, migration := range migrations {
		loaded, err := loadMigrationAssets(migration.FS, migration.Path)
		if err != nil {
			return nil, err
		}
		for _, asset := range loaded {
			key := asset.databaseType + "\x00" + asset.dataSource + "\x00" + asset.versionName
			if assetKeys[key] {
				return nil, fmt.Errorf(
					"迁移模块 %s 数据库类型 %s 数据源 %s 版本 %s 存在重复资源",
					name,
					asset.databaseType,
					asset.dataSource,
					asset.versionName,
				)
			}
			assetKeys[key] = true
			assets = append(assets, asset)
		}
	}
	assetsByModule[name] = assets
	return assets, nil
}

// runModule 按依赖顺序执行迁移模块。
func (r *Runner) runModule(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	name ModuleName,
	dataSource string,
	targetClient *databaseGorm.Client,
	visited map[ModuleName]bool,
	assetsByModule map[ModuleName][]migrationAsset,
) error {
	if visited[name] {
		return nil
	}
	migrations := r.registry.migrations[name]
	if len(migrations) == 0 {
		return fmt.Errorf("迁移模块未提供资源: %s", name)
	}
	var err error
	for _, dependency := range migrationDependencies(migrations) {
		err = r.runModule(ctx, centralClient, dependency, dataSource, targetClient, visited, assetsByModule)
		if err != nil {
			return err
		}
	}
	for _, asset := range assetsByModule[name] {
		if asset.dataSource != dataSource || asset.databaseType != targetClient.Driver() {
			continue
		}
		err = r.runMigration(ctx, centralClient, targetClient, name, asset)
		if err != nil {
			return err
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
	moduleName ModuleName,
	asset migrationAsset,
) error {
	if targetClient == nil || targetClient.DB == nil {
		return fmt.Errorf("迁移模块 %s 数据库客户端不能为空", moduleName)
	}
	centralDriver := centralClient.Driver()
	if centralDriver != databaseTypeMySQL {
		return fmt.Errorf("迁移记录数据源暂不支持数据库驱动 %s", centralDriver)
	}
	targetDriver := targetClient.Driver()
	if targetDriver != databaseTypeMySQL && targetDriver != databaseTypeDoris {
		return fmt.Errorf("迁移模块 %s 暂不支持数据库驱动 %s", moduleName, targetDriver)
	}
	if asset.databaseType != targetDriver {
		return fmt.Errorf(
			"迁移模块 %s 数据源 %s 的数据库类型为 %s，不能执行 %s 脚本",
			moduleName,
			asset.dataSource,
			targetDriver,
			asset.databaseType,
		)
	}
	err := withMigrationLock(ctx, centralClient, moduleName, asset.dataSource, func() error {
		return applyMigrationAssets(ctx, centralClient, targetClient, moduleName, asset.dataSource, []migrationAsset{asset})
	})
	if err != nil {
		return fmt.Errorf("执行迁移模块 %s 失败: %w", moduleName, err)
	}
	return nil
}

// Ready 表示默认数据库已完成迁移，可供业务模块建立依赖。
type Ready struct{}
