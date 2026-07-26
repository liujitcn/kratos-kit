package migration

import (
	"context"
	"fmt"
	"strings"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// applyMigrationAssets 按版本顺序执行脚本并写入集中迁移记录。
func applyMigrationAssets(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	targetClient *databaseGorm.Client,
	module ModuleName,
	dataSource string,
	assets []migrationAsset,
) error {
	var applied map[string]struct{}
	var err error
	applied, err = loadAppliedMigrations(ctx, centralClient, module, dataSource)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if _, exists := applied[asset.versionName]; exists {
			continue
		}
		upSQL := make([]string, 0, len(asset.upScripts))
		for _, script := range asset.upScripts {
			upSQL = append(upSQL, string(script.sql))
		}
		downSQL := make([]string, 0, len(asset.downScripts))
		for _, script := range asset.downScripts {
			downSQL = append(downSQL, string(script.sql))
		}
		err = targetClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, script := range asset.upScripts {
				result := tx.Exec(string(script.sql))
				if result.Error != nil {
					return fmt.Errorf("执行迁移版本 %s 文件 %s 失败: %w", asset.versionName, script.name, result.Error)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("执行迁移版本 %s 失败: %w", asset.versionName, err)
		}
		history := &baseMigration{
			Module:      module.String(),
			DataSource:  dataSource,
			Version:     asset.versionName,
			UpSql:       strings.Join(upSQL, "\n\n"),
			DownSql:     strings.Join(downSQL, "\n\n"),
			Description: asset.description,
		}
		err = centralClient.WithContext(ctx).Create(history).Error
		if err != nil {
			return fmt.Errorf("记录迁移版本 %s 失败: %w", asset.versionName, err)
		}
	}
	return nil
}

// loadAppliedMigrations 查询一个迁移数据源已经记录的版本。
func loadAppliedMigrations(
	ctx context.Context,
	client *databaseGorm.Client,
	module ModuleName,
	dataSource string,
) (map[string]struct{}, error) {
	histories := make([]baseMigration, 0)
	query := client.WithContext(ctx).
		Where(&baseMigration{DataSource: dataSource}).
		Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "version"}},
		}})
	var err error
	err = query.Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("读取迁移版本记录失败: %w", err)
	}
	applied := make(map[string]struct{}, len(histories))
	for _, history := range histories {
		if history.Module != "" && history.Module != module.String() {
			continue
		}
		applied[history.Version] = struct{}{}
	}
	return applied, nil
}
