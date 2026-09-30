package gorm

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// binarySizeConstraintPrefix PostgreSQL 二进制列长度检查约束的统一前缀，
// 生成器依赖该前缀从约束定义找回 size 标签。
const binarySizeConstraintPrefix = "gormsize_"

// applyDialectBinarySizes 在 PostgreSQL 上为带 size 的二进制列补长度检查约束。
//
// MySQL 的 varbinary(N) 自带长度并作为索引键长依据，PostgreSQL 的 bytea 没有长度概念；
// 该约束既让两库获得一致的长度校验，也让生成器能从约束定义找回 size 标签，
// 保证两个源库生成的模型一致、两库可由同一份模型互相创建。
func applyDialectBinarySizes(db *gorm.DB, models []interface{}) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" || len(models) == 0 {
		return nil
	}
	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			return fmt.Errorf("parse model to apply binary size: %w", err)
		}
		for _, field := range stmt.Schema.Fields {
			if field.DataType != schema.Bytes {
				continue
			}
			size, err := strconv.Atoi(field.TagSettings["SIZE"])
			if err != nil || size <= 0 {
				continue
			}
			if err = ensureBinarySizeConstraint(db, stmt.Schema.Table, field.DBName, size); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureBinarySizeConstraint 确保表上存在指定长度的二进制列检查约束，长度变化时重建。
func ensureBinarySizeConstraint(db *gorm.DB, table, column string, size int) error {
	if !isPlainIdentifier(table) || !isPlainIdentifier(column) {
		return fmt.Errorf("二进制列长度约束的表列名含非法字符: %s.%s", table, column)
	}
	constraintName := binarySizeConstraintPrefix + table + "_" + column
	var definition string
	err := db.Raw(`SELECT pg_get_constraintdef(con.oid) FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		WHERE con.conname = ? AND c.relname = ? AND con.contype = 'c'`, constraintName, table).Scan(&definition).Error
	if err != nil {
		return fmt.Errorf("查询二进制列长度约束: %w", err)
	}
	if extractBinarySizeConstraint(definition) == size {
		return nil
	}
	// 定义缺失或不一致（模型长度调整）时重建约束。
	if definition != "" {
		if err = db.Exec(fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT %s`, table, constraintName)).Error; err != nil {
			return fmt.Errorf("删除旧二进制列长度约束: %w", err)
		}
	}
	sql := fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s CHECK (octet_length(%s) <= %d)`, table, constraintName, column, size)
	if err = db.Exec(sql).Error; err != nil {
		return fmt.Errorf("创建二进制列长度约束: %w", err)
	}
	return nil
}

// extractBinarySizeConstraint 从约束定义中解析长度，格式不符返回 0。
func extractBinarySizeConstraint(definition string) int {
	// 自建约束定义形如 CHECK (octet_length("digest") <= 32)。
	index := strings.Index(definition, "<= ")
	if index < 0 || !strings.Contains(definition, "octet_length") {
		return 0
	}
	size, err := strconv.Atoi(strings.TrimSpace(definition[index+3:]))
	if err != nil {
		return 0
	}
	return size
}

// isPlainIdentifier 校验标识符只含字母数字下划线，保证拼接 DDL 安全。
func isPlainIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
