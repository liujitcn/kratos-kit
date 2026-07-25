package migration

import (
	"context"
	"fmt"

	databaseGorm "github.com/liujitcn/kratos-kit/database/gorm"
	"gorm.io/gorm/clause"
)

// applyMigrationAssets 按版本顺序执行脚本并写入集中迁移记录。
func applyMigrationAssets(
	ctx context.Context,
	centralClient *databaseGorm.Client,
	targetClient *databaseGorm.Client,
	business string,
	assets []migrationAsset,
) error {
	var applied map[string]bool
	var err error
	applied, err = loadAppliedMigrations(ctx, centralClient, business)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		status, exists := applied[asset.versionName]
		if exists {
			if status != true {
				return fmt.Errorf("迁移版本 %s 处于失败状态", asset.versionName)
			}
			continue
		}
		history := &baseMigration{
			Business:    business,
			Version:     asset.versionName,
			Description: asset.description,
			IsSuccess:   false,
		}
		err = centralClient.WithContext(ctx).Create(history).Error
		if err != nil {
			return fmt.Errorf("记录迁移版本 %s 开始状态失败: %w", asset.versionName, err)
		}
		err = targetClient.WithContext(ctx).Exec(string(asset.sql)).Error
		if err != nil {
			return fmt.Errorf("执行迁移版本 %s (%s) 失败: %w", asset.versionName, asset.name, err)
		}
		result := centralClient.WithContext(ctx).
			Model(&baseMigration{}).
			Where(&baseMigration{Business: business, Version: asset.versionName}).
			Updates(map[string]interface{}{
				"status": true,
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

// loadAppliedMigrations 查询一个迁移业务已经完成或失败的版本。
func loadAppliedMigrations(
	ctx context.Context,
	client *databaseGorm.Client,
	business string,
) (map[string]bool, error) {
	histories := make([]baseMigration, 0)
	query := client.WithContext(ctx).
		Where(&baseMigration{Business: business}).
		Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "version"}},
		}})
	var err error
	err = query.Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("读取迁移版本记录失败: %w", err)
	}
	applied := make(map[string]bool)
	for _, history := range histories {
		applied[history.Version] = history.IsSuccess
	}
	return applied, nil
}
