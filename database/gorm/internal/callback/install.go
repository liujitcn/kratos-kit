package callback

import (
	"fmt"

	"gorm.io/gorm"
)

// Install 集中安装内置能力，再安装当前客户端创建前登记的扩展回调。
func Install(db *gorm.DB) error {
	definitions := []callbackDefinition{
		{operation: "query", name: "kratos:data_scope:query", before: "gorm:query", handler: addDataScopeWhere},
		{operation: "query", name: "kratos:tenant:query", before: "gorm:query", after: "kratos:data_scope:query", handler: addTenantWhere},
		{operation: "row", name: "kratos:data_scope:row", before: "gorm:row", handler: addDataScopeWhere},
		{operation: "row", name: "kratos:tenant:row", before: "gorm:row", after: "kratos:data_scope:row", handler: addTenantWhere},
		{operation: "raw", name: "kratos:isolation:raw", before: "gorm:raw", handler: rejectRawDataIsolation},
		{operation: "create", name: "kratos:audit:create", before: "gorm:before_create", handler: fillCreatedFields},
		{operation: "create", name: "kratos:tenant:create", before: "gorm:before_create", after: "kratos:audit:create", handler: fillTenantID},
		{operation: "update", name: "kratos:audit:update", before: "gorm:before_update", handler: fillUpdatedFields},
		{operation: "update", name: "kratos:data_scope:update", before: "gorm:update", after: "gorm:before_update", handler: addDataScopeWhere},
		{operation: "update", name: "kratos:tenant:update", before: "gorm:update", after: "kratos:data_scope:update", handler: addTenantWhere},
		{operation: "delete", name: "kratos:data_scope:delete", before: "gorm:delete", handler: addDataScopeWhere},
		{operation: "delete", name: "kratos:tenant:delete", before: "gorm:delete", after: "kratos:data_scope:delete", handler: addTenantWhere},
	}
	return installCallbacks(db, append(definitions, customCallbackSnapshot()...))
}

// installCallbacks 将声明式回调绑定到 GORM，统一保留阶段约束与注册错误上下文。
func installCallbacks(db *gorm.DB, definitions []callbackDefinition) error {
	for _, definition := range definitions {
		processor := db.Callback().Query()
		switch definition.operation {
		case "query":
		case "row":
			processor = db.Callback().Row()
		case "raw":
			processor = db.Callback().Raw()
		case "create":
			processor = db.Callback().Create()
		case "update":
			processor = db.Callback().Update()
		case "delete":
			processor = db.Callback().Delete()
		default:
			return fmt.Errorf("不支持的回调操作: %s", definition.operation)
		}
		callback := processor.Before(definition.before)
		if definition.after != "" {
			callback = callback.After(definition.after)
		}
		err := callback.Register(definition.name, definition.handler)
		if err != nil {
			return fmt.Errorf("注册回调 %s: %w", definition.name, err)
		}
	}
	return nil
}
