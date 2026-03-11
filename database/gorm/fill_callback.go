package gorm

import (
	"slices"
	"time"

	"github.com/go-kratos/kratos/v2/log"
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

func fillCreatedFields(db *gorm.DB) {
	table := db.Statement.Table
	if slices.Contains(auditExcludeTables, table) {
		return
	}

	var userId int64
	ctx := db.Statement.Context
	if userInfo, err := auth.FromContext(ctx); err == nil {
		log.Warnf("context has no user info, use default user id")
	} else {
		if userInfo == nil {
			log.Errorf("get user id failed, use default user id")
		} else {
			userId = userInfo.UserId
		}
	}

	now := time.Now()
	safeSetColumn(db, "CreatedBy", userId)
	safeSetColumn(db, "UpdatedBy", userId)
	safeSetColumn(db, "CreatedAt", now)
	safeSetColumn(db, "UpdatedAt", now)
}

func fillUpdatedFields(db *gorm.DB) {
	table := db.Statement.Table
	if slices.Contains(auditExcludeTables, table) {
		return
	}

	var userId int64
	ctx := db.Statement.Context
	if userInfo, err := auth.FromContext(ctx); err == nil {
		log.Warnf("context has no user info, use default user id")
	} else {
		if userInfo == nil {
			log.Errorf("get user id failed, use default user id")
		} else {
			userId = userInfo.UserId
		}
	}

	safeSetColumn(db, "UpdatedBy", userId)
	safeSetColumn(db, "UpdatedAt", time.Now())
}
