package ent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"entgo.io/ent/dialect"
)

// TableComment 描述 Ent 数据库表注释。
type TableComment struct {
	Table   string
	Comment string
}

var (
	registeredTableCommentsMu sync.RWMutex
	registeredTableComments   []TableComment
)

// RegisterTableComment 注册用于迁移后回填的数据库表注释。
func RegisterTableComment(table string, comment string) {
	if table == "" || comment == "" {
		return
	}
	registeredTableCommentsMu.Lock()
	defer registeredTableCommentsMu.Unlock()
	registeredTableComments = append(registeredTableComments, TableComment{
		Table:   table,
		Comment: comment,
	})
}

// RegisterTableComments 注册多个用于迁移后回填的数据库表注释。
func RegisterTableComments(comments ...TableComment) {
	if len(comments) == 0 {
		return
	}
	registeredTableCommentsMu.Lock()
	defer registeredTableCommentsMu.Unlock()
	for _, comment := range comments {
		if comment.Table == "" || comment.Comment == "" {
			continue
		}
		registeredTableComments = append(registeredTableComments, comment)
	}
}

// RunRegisteredTableComments 执行已注册的表注释回填。
func (c *Client) RunRegisteredTableComments(ctx context.Context) error {
	comments := getRegisteredTableComments()
	for _, comment := range comments {
		err := c.applyTableComment(ctx, comment)
		if err != nil {
			return err
		}
	}
	return nil
}

// getRegisteredTableComments 返回已注册的表注释副本。
func getRegisteredTableComments() []TableComment {
	registeredTableCommentsMu.RLock()
	defer registeredTableCommentsMu.RUnlock()
	if len(registeredTableComments) == 0 {
		return nil
	}
	dup := make([]TableComment, len(registeredTableComments))
	copy(dup, registeredTableComments)
	return dup
}

// applyTableComment 根据不同 SQL 方言执行表注释语句。
func (c *Client) applyTableComment(ctx context.Context, comment TableComment) error {
	table, err := quoteTableName(c.dialectName, comment.Table)
	if err != nil {
		return err
	}

	switch c.dialectName {
	case dialect.MySQL:
		_, err = c.sqlDB.ExecContext(ctx, "ALTER TABLE "+table+" COMMENT = "+quoteStringLiteral(comment.Comment))
	case dialect.Postgres:
		_, err = c.sqlDB.ExecContext(ctx, "COMMENT ON TABLE "+table+" IS "+quoteStringLiteral(comment.Comment))
	case dialect.SQLite:
		return nil
	default:
		err = fmt.Errorf("ent table comment unsupported dialect: %s", c.dialectName)
	}
	if err != nil {
		return err
	}
	return nil
}

// quoteTableName 引用数据库表名，支持 schema.table 形式。
func quoteTableName(dialectName string, table string) (string, error) {
	parts := strings.Split(table, ".")
	for i, part := range parts {
		if !isSafeIdentifier(part) {
			return "", fmt.Errorf("unsafe table identifier: %s", table)
		}
		parts[i] = quoteIdentifier(dialectName, part)
	}
	return strings.Join(parts, "."), nil
}

// quoteIdentifier 按 SQL 方言引用单个标识符。
func quoteIdentifier(dialectName string, name string) string {
	if dialectName == dialect.MySQL {
		return "`" + name + "`"
	}
	return `"` + name + `"`
}

// quoteStringLiteral 转义 SQL 字符串字面量。
func quoteStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// isSafeIdentifier 判断表名片段是否只包含常规标识符字符。
func isSafeIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '_' {
			continue
		}
		return false
	}
	return true
}
