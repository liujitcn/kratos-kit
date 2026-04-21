package gorm

import (
	"context"
	"slices"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	"github.com/liujitcn/kratos-kit/auth"
	"gorm.io/gorm"
)

func init() {
	RegisterCallbackCreate(fillCreatedFields)
	RegisterCallbackUpdate(fillUpdatedFields)
}

var auditExcludeTables = []string{
	"base_log",
}

func safeSetColumn(db *gorm.DB, fieldName string, value interface{}) {
	if !isFieldZero(db, fieldName) {
		return
	}
	db.Statement.SetColumn(fieldName, value)
}

func isFieldZero(db *gorm.DB, fieldName string) bool {
	statement := db.Statement
	if statement == nil {
		return false
	}
	if field := statement.Schema.LookUpField(fieldName); field == nil {
		return false
	}
	return true
}

// hasAnyField 判断当前模型是否声明了任一指定字段。
func hasAnyField(db *gorm.DB, fieldNames ...string) bool {
	statement := db.Statement
	if statement == nil || statement.Schema == nil {
		return false
	}

	for _, fieldName := range fieldNames {
		if statement.Schema.LookUpField(fieldName) != nil {
			return true
		}
	}

	return false
}

// getUserIDFromContext 从上下文中解析当前用户ID。
func getUserIDFromContext(ctx context.Context) int64 {
	if ctx == nil || ctx == context.Background() || ctx == context.TODO() || isAppLifecycleContext(ctx) {
		return 0
	}

	userInfo, err := auth.FromContext(ctx)
	if err != nil {
		log.Warnf("context has no user info, use default user id")
		return 0
	}
	if userInfo == nil {
		log.Errorf("get user id failed, use default user id")
		return 0
	}

	return userInfo.UserId
}

// fillCreatedFields 在创建时回填审计字段。
func fillCreatedFields(db *gorm.DB) {
	table := db.Statement.Table
	if slices.Contains(auditExcludeTables, table) {
		return
	}

	var userId int64
	// 仅当模型声明了审计人字段时，才尝试从上下文中解析用户ID，避免无效读取。
	if hasAnyField(db, "CreatedBy", "UpdatedBy") {
		userId = getUserIDFromContext(db.Statement.Context)
	}

	now := time.Now()
	safeSetColumn(db, "CreatedBy", userId)
	safeSetColumn(db, "UpdatedBy", userId)
	safeSetColumn(db, "CreatedAt", now)
	safeSetColumn(db, "UpdatedAt", now)
}

// fillUpdatedFields 在更新时回填审计字段。
func fillUpdatedFields(db *gorm.DB) {
	table := db.Statement.Table
	if slices.Contains(auditExcludeTables, table) {
		return
	}

	var userId int64
	// 更新时只在存在 UpdatedBy 字段的模型上解析用户ID，避免不必要的上下文访问。
	if hasAnyField(db, "UpdatedBy") {
		userId = getUserIDFromContext(db.Statement.Context)
	}

	safeSetColumn(db, "UpdatedBy", userId)
	safeSetColumn(db, "UpdatedAt", time.Now())
}

// isAppLifecycleContext 判断当前上下文是否为 Kratos 应用生命周期上下文。
func isAppLifecycleContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	// 请求上下文会额外挂载 transport 信息，不能按应用生命周期上下文处理。
	if _, ok := kratosTransport.FromServerContext(ctx); ok {
		return false
	}

	_, ok := kratos.FromContext(ctx)
	return ok
}
