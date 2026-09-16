package callback

import (
	"fmt"
	"slices"
	"sync"

	"gorm.io/gorm"
)

var (
	registeredCallbackMu sync.RWMutex
	customCallbacks      []callbackDefinition
)

// callbackDefinition 描述单个数据库操作上的回调及其执行位置。
type callbackDefinition struct {
	operation string
	name      string
	before    string
	after     string
	handler   func(*gorm.DB)
}

// RegisterCallbackQuery 注册查询钩子，必须在创建客户端前注册。
func RegisterCallbackQuery(fn func(*gorm.DB)) {
	registerCustomCallbacks("query", "gorm:query", "", fn)
}

// RegisterCallbackQueries 批量注册查询钩子，必须在创建客户端前注册。
func RegisterCallbackQueries(fn ...func(*gorm.DB)) {
	registerCustomCallbacks("query", "gorm:query", "", fn...)
}

// RegisterCallbackQueryAfter 注册查询完成后的钩子，必须在创建客户端前注册。
func RegisterCallbackQueryAfter(fn func(*gorm.DB)) {
	registerCustomCallbacks("query", "", "gorm:after_query", fn)
}

// RegisterCallbackRow 注册单行和流式查询钩子，必须在创建客户端前注册。
func RegisterCallbackRow(fn func(*gorm.DB)) {
	registerCustomCallbacks("row", "gorm:row", "", fn)
}

// RegisterCallbackRaw 注册原生 SQL 钩子，必须在创建客户端前注册。
func RegisterCallbackRaw(fn func(*gorm.DB)) {
	registerCustomCallbacks("raw", "gorm:raw", "", fn)
}

// RegisterCallbackCreate 注册创建钩子，必须在创建客户端前注册。
func RegisterCallbackCreate(fn func(*gorm.DB)) {
	registerCustomCallbacks("create", "gorm:before_create", "", fn)
}

// RegisterCallbackCreates 批量注册创建钩子，必须在创建客户端前注册。
func RegisterCallbackCreates(fn ...func(*gorm.DB)) {
	registerCustomCallbacks("create", "gorm:before_create", "", fn...)
}

// RegisterCallbackCreateAfter 注册创建完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackCreateAfter(fn func(*gorm.DB)) {
	registerCustomCallbacks("create", "gorm:commit_or_rollback_transaction", "gorm:after_create", fn)
}

// RegisterCallbackUpdate 注册更新钩子，必须在创建客户端前注册。
func RegisterCallbackUpdate(fn func(*gorm.DB)) {
	registerCustomCallbacks("update", "gorm:before_update", "", fn)
}

// RegisterCallbackUpdates 批量注册更新钩子，必须在创建客户端前注册。
func RegisterCallbackUpdates(fn ...func(*gorm.DB)) {
	registerCustomCallbacks("update", "gorm:before_update", "", fn...)
}

// RegisterCallbackUpdateAfter 注册更新完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackUpdateAfter(fn func(*gorm.DB)) {
	registerCustomCallbacks("update", "gorm:commit_or_rollback_transaction", "gorm:after_update", fn)
}

// RegisterCallbackDelete 注册删除钩子，必须在创建客户端前注册。
func RegisterCallbackDelete(fn func(*gorm.DB)) {
	registerCustomCallbacks("delete", "gorm:delete", "", fn)
}

// RegisterCallbackDeletes 批量注册删除钩子，必须在创建客户端前注册。
func RegisterCallbackDeletes(fn ...func(*gorm.DB)) {
	registerCustomCallbacks("delete", "gorm:delete", "", fn...)
}

// RegisterCallbackDeleteAfter 注册删除完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackDeleteAfter(fn func(*gorm.DB)) {
	registerCustomCallbacks("delete", "gorm:commit_or_rollback_transaction", "gorm:after_delete", fn)
}

// RegisterCallbackUpdateBefore 注册指定更新节点前执行的钩子，必须在创建客户端前注册。
func RegisterCallbackUpdateBefore(anchor string, fn func(*gorm.DB)) {
	if anchor == "" {
		anchor = "gorm:before_update"
	}
	registerCustomCallbacks("update", anchor, "", fn)
}

// registerCustomCallbacks 在同一把锁下登记扩展回调，统一忽略空处理器。
func registerCustomCallbacks(operation, before, after string, handlers ...func(*gorm.DB)) {
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		customCallbacks = append(customCallbacks, callbackDefinition{
			operation: operation,
			name:      fmt.Sprintf("kratos:custom:%s:%d", operation, len(customCallbacks)),
			before:    before,
			after:     after,
			handler:   handler,
		})
	}
}

// customCallbackSnapshot 一次性取得当前扩展回调，避免客户端安装期间混入后续注册项。
func customCallbackSnapshot() []callbackDefinition {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	return slices.Clone(customCallbacks)
}
