package migration

import (
	"context"
	"fmt"
	"strings"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// appliedMigration 保存已记录迁移的状态及其兼容信息。
type appliedMigration struct {
	// id 是迁移记录主键，用于兼容旧记录更新状态。
	id int64
	// success 表示迁移是否已成功执行。
	success bool
}

// applyMigrationAssets 按版本顺序执行脚本并写入集中迁移记录。
func applyMigrationAssets(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	targetClient *databaseGorm.Client,
	module ModuleName,
	dataSource string,
	assets []migrationAsset,
) error {
	var applied map[string]appliedMigration
	var err error
	applied, err = loadAppliedMigrations(ctx, centralClient, module, dataSource)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		history, exists := applied[asset.versionName]
		if exists && history.success {
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
		if !exists {
			history := &baseMigration{
				Module:      module.String(),
				DataSource:  dataSource,
				Version:     asset.versionName,
				UpSql:       strings.Join(upSQL, "\n\n"),
				DownSql:     strings.Join(downSQL, "\n\n"),
				Description: asset.description,
				IsSuccess:   false,
			}
			err = centralClient.WithContext(ctx).Create(history).Error
			if err != nil {
				return fmt.Errorf("记录迁移版本 %s 开始状态失败: %w", asset.versionName, err)
			}
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
			// 保留失败记录，允许应用先启动；后续启动时会继续重试该版本。
			return nil
		}
		query := centralClient.WithContext(ctx).Model(&baseMigration{})
		if exists {
			query = query.Where(&baseMigration{ID: history.id})
		} else {
			query = query.Where(&baseMigration{Module: module.String(), DataSource: dataSource, Version: asset.versionName})
		}
		result := query.Updates(map[string]interface{}{
			"is_success": true,
		})
		if result.Error != nil {
			err = result.Error
			return fmt.Errorf("记录迁移版本 %s 完成状态失败: %w", asset.versionName, err)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("记录迁移版本 %s 完成状态失败: 更新记录数为 %d", asset.versionName, result.RowsAffected)
		}
	}
	return nil
}

// loadAppliedMigrations 查询一个迁移数据源已经完成或失败的版本。
func loadAppliedMigrations(
	ctx context.Context,
	client *databaseGorm.Client,
	module ModuleName,
	dataSource string,
) (map[string]appliedMigration, error) {
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
	applied := make(map[string]appliedMigration)
	for _, history := range histories {
		if history.Module != module.String() {
			continue
		}
		applied[history.Version] = appliedMigration{
			id:      history.ID,
			success: history.IsSuccess,
		}
	}
	for _, history := range histories {
		if history.Module != "" {
			continue
		}
		if _, exists := applied[history.Version]; exists {
			continue
		}
		// 旧版本没有 module 字段，按当前模块兼容读取，避免重复执行已有脚本。
		applied[history.Version] = appliedMigration{
			id:      history.ID,
			success: history.IsSuccess,
		}
	}
	return applied, nil
}
