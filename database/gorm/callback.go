package gorm

import (
	"github.com/liujitcn/kratos-kit/database/gorm/internal/callback"
	"gorm.io/gorm"
)

// ProjectScope 是服务端解析后的租户项目范围，[0] 表示对应租户全部项目。
type ProjectScope = callback.ProjectScope

// ProjectScopeLoader 从可信身份实时加载项目范围，失败时禁止访问。
type ProjectScopeLoader = callback.ProjectScopeLoader

const (
	// DefaultTenantCode 表示拥有跨租户访问能力的默认租户编码。
	DefaultTenantCode = callback.DefaultTenantCode
	// DataScopeUnknown 表示未声明角色数据范围。
	DataScopeUnknown = callback.DataScopeUnknown
	// DataScopeAll 表示角色拥有全部数据范围。
	DataScopeAll = callback.DataScopeAll
	// DataScopeDeptAndChildren 表示角色拥有本部门及子部门数据范围。
	DataScopeDeptAndChildren = callback.DataScopeDeptAndChildren
	// DataScopeSelfDept 表示角色仅拥有本部门数据范围。
	DataScopeSelfDept = callback.DataScopeSelfDept
	// DataScopeSelfUser 表示角色仅拥有本人数据范围。
	DataScopeSelfUser = callback.DataScopeSelfUser
)

var (
	// ErrTenantContextMissing 表示租户表操作缺少有效的租户身份。
	ErrTenantContextMissing = callback.ErrTenantContextMissing
	// ErrDataScopeContextMissing 表示数据权限表操作缺少有效的用户身份。
	ErrDataScopeContextMissing = callback.ErrDataScopeContextMissing
	// ErrRawDataIsolationUnsupported 表示原生 SQL 无法安全追加数据隔离条件。
	ErrRawDataIsolationUnsupported = callback.ErrRawDataIsolationUnsupported
	// ErrProjectScopeDenied 表示项目范围缺失、非法或写入越权。
	ErrProjectScopeDenied = callback.ErrProjectScopeDenied
)

// SkipDataIsolation 在独立会话中跳过租户、角色、项目和原生 SQL 隔离，仅供可信系统任务使用。
func SkipDataIsolation(db *gorm.DB) *gorm.DB {
	return callback.SkipDataIsolation(db)
}

// RegisterProjectIsolation 为指定数据库的业务表注册项目隔离，表与字段必须由宿主显式提供。
func RegisterProjectIsolation(db *gorm.DB, columns map[string]string, load ProjectScopeLoader) error {
	return callback.RegisterProjectIsolation(db, columns, load)
}

// RegisterCallbackQuery 注册查询钩子，必须在创建客户端前注册。
func RegisterCallbackQuery(fn func(*gorm.DB)) {
	callback.RegisterCallbackQuery(fn)
}

// RegisterCallbackQueries 批量注册查询钩子，必须在创建客户端前注册。
func RegisterCallbackQueries(fn ...func(*gorm.DB)) {
	callback.RegisterCallbackQueries(fn...)
}

// RegisterCallbackQueryAfter 注册查询完成后的钩子，必须在创建客户端前注册。
func RegisterCallbackQueryAfter(fn func(*gorm.DB)) {
	callback.RegisterCallbackQueryAfter(fn)
}

// RegisterCallbackRow 注册单行和流式查询钩子，必须在创建客户端前注册。
func RegisterCallbackRow(fn func(*gorm.DB)) {
	callback.RegisterCallbackRow(fn)
}

// RegisterCallbackRaw 注册原生 SQL 钩子，必须在创建客户端前注册。
func RegisterCallbackRaw(fn func(*gorm.DB)) {
	callback.RegisterCallbackRaw(fn)
}

// RegisterCallbackCreate 注册创建钩子，必须在创建客户端前注册。
func RegisterCallbackCreate(fn func(*gorm.DB)) {
	callback.RegisterCallbackCreate(fn)
}

// RegisterCallbackCreates 批量注册创建钩子，必须在创建客户端前注册。
func RegisterCallbackCreates(fn ...func(*gorm.DB)) {
	callback.RegisterCallbackCreates(fn...)
}

// RegisterCallbackCreateAfter 注册创建完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackCreateAfter(fn func(*gorm.DB)) {
	callback.RegisterCallbackCreateAfter(fn)
}

// RegisterCallbackUpdate 注册更新钩子，必须在创建客户端前注册。
func RegisterCallbackUpdate(fn func(*gorm.DB)) {
	callback.RegisterCallbackUpdate(fn)
}

// RegisterCallbackUpdates 批量注册更新钩子，必须在创建客户端前注册。
func RegisterCallbackUpdates(fn ...func(*gorm.DB)) {
	callback.RegisterCallbackUpdates(fn...)
}

// RegisterCallbackUpdateAfter 注册更新完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackUpdateAfter(fn func(*gorm.DB)) {
	callback.RegisterCallbackUpdateAfter(fn)
}

// RegisterCallbackDelete 注册删除钩子，必须在创建客户端前注册。
func RegisterCallbackDelete(fn func(*gorm.DB)) {
	callback.RegisterCallbackDelete(fn)
}

// RegisterCallbackDeletes 批量注册删除钩子，必须在创建客户端前注册。
func RegisterCallbackDeletes(fn ...func(*gorm.DB)) {
	callback.RegisterCallbackDeletes(fn...)
}

// RegisterCallbackDeleteAfter 注册删除完成且事务提交前的钩子，必须在创建客户端前注册。
func RegisterCallbackDeleteAfter(fn func(*gorm.DB)) {
	callback.RegisterCallbackDeleteAfter(fn)
}

// RegisterCallbackUpdateBefore 注册指定更新节点前执行的钩子，必须在创建客户端前注册。
func RegisterCallbackUpdateBefore(anchor string, fn func(*gorm.DB)) {
	callback.RegisterCallbackUpdateBefore(anchor, fn)
}
