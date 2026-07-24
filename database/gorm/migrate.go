package gorm

import (
	"fmt"
	"strings"
	"sync"

	gormdb "gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	registeredMigrateModelsMu      sync.RWMutex
	registeredMigrateModels        []interface{}
	registeredMigrateModelsVersion uint64
	isolationTableCache            sync.Map
)

const migrateRegistryKey = "kratos-kit:gorm:migrate-registry"

var globalMigrateRegistryMarker = new(struct{})

type migrateRegistry struct {
	models   []interface{}
	explicit bool
	version  uint64
}

type isolationTableCacheEntry struct {
	version         uint64
	tenantTables    map[string]struct{}
	dataScopeTables map[string]struct{}
	auditFields     map[string]map[string]auditFieldMetadata
	err             error
}

type auditFieldMetadata struct {
	name   string
	dbName string
}

// RegisterMigrateModel 注册用于数据库迁移的数据库模型。
func RegisterMigrateModel(model interface{}) {
	if model == nil {
		return
	}
	registeredMigrateModelsMu.Lock()
	defer registeredMigrateModelsMu.Unlock()
	registeredMigrateModels = append(registeredMigrateModels, model)
	registeredMigrateModelsVersion++
}

// RegisterMigrateModels 注册用于数据库迁移的数据库模型。
func RegisterMigrateModels(models ...interface{}) {
	if len(models) == 0 {
		return
	}
	registeredMigrateModelsMu.Lock()
	defer registeredMigrateModelsMu.Unlock()
	registeredMigrateModels = append(registeredMigrateModels, models...)
	registeredMigrateModelsVersion++
}

// getRegisteredMigrateModels 返回已注册的包级模型副本（线程安全）。
func getRegisteredMigrateModels() []interface{} {
	models, _ := getRegisteredMigrateModelsSnapshot()
	return models
}

// newMigrateRegistry 创建当前客户端使用的模型注册范围。
func newMigrateRegistry(models []interface{}, explicit bool) *migrateRegistry {
	return &migrateRegistry{
		models:   append([]interface{}(nil), models...),
		explicit: explicit,
		version:  1,
	}
}

// getMigrateModels 返回当前客户端对应的模型副本。
func getMigrateModels(db *gormdb.DB) []interface{} {
	models, _ := getMigrateModelsSnapshot(db)
	return models
}

// getMigrateModelsSnapshot 返回当前客户端模型副本及版本。
func getMigrateModelsSnapshot(db *gormdb.DB) ([]interface{}, uint64) {
	if db != nil && db.Statement != nil {
		if value, ok := db.Get(migrateRegistryKey); ok {
			registry, registryOK := value.(*migrateRegistry)
			if registryOK && registry.explicit {
				return append([]interface{}(nil), registry.models...), registry.version
			}
		}
	}
	return getRegisteredMigrateModelsSnapshot()
}

// getRegisteredMigrateModelsSnapshot 返回注册模型副本及其版本。
func getRegisteredMigrateModelsSnapshot() ([]interface{}, uint64) {
	registeredMigrateModelsMu.RLock()
	defer registeredMigrateModelsMu.RUnlock()
	if len(registeredMigrateModels) == 0 {
		return nil, registeredMigrateModelsVersion
	}
	dup := make([]interface{}, len(registeredMigrateModels))
	copy(dup, registeredMigrateModels)
	return dup, registeredMigrateModelsVersion
}

// getRegisteredMigrateModelsVersion 返回当前注册模型版本。
func getRegisteredMigrateModelsVersion() uint64 {
	registeredMigrateModelsMu.RLock()
	defer registeredMigrateModelsMu.RUnlock()
	return registeredMigrateModelsVersion
}

// getMigrateRegistryIdentity 返回缓存需要区分的模型注册范围标识。
func getMigrateRegistryIdentity(db *gormdb.DB) interface{} {
	if db != nil && db.Statement != nil {
		if value, ok := db.Get(migrateRegistryKey); ok {
			registry, registryOK := value.(*migrateRegistry)
			if registryOK && registry.explicit {
				return registry
			}
		}
	}
	return globalMigrateRegistryMarker
}

// getIsolationTables 返回当前命名策略下缓存的租户表和数据范围表。
func getIsolationTables(db *gormdb.DB) (map[string]struct{}, map[string]struct{}, error) {
	entry := getIsolationTableCacheEntry(db)
	return entry.tenantTables, entry.dataScopeTables, entry.err
}

// getRegisteredAuditField 返回当前语句目标表已注册的审计字段元数据。
func getRegisteredAuditField(db *gormdb.DB, fieldName string) (auditFieldMetadata, bool, error) {
	entry := getIsolationTableCacheEntry(db)
	if entry.err != nil {
		return auditFieldMetadata{}, false, entry.err
	}
	if db == nil || db.Statement == nil {
		return auditFieldMetadata{}, false, nil
	}
	if tableFields, tableRegistered := entry.auditFields[db.Statement.Table]; tableRegistered {
		field, fieldExists := tableFields[fieldName]
		return field, fieldExists, nil
	}
	if db.Statement.TableExpr == nil {
		return auditFieldMetadata{}, false, nil
	}
	tableExprFields := strings.Fields(db.Statement.TableExpr.SQL)
	if len(tableExprFields) == 0 {
		return auditFieldMetadata{}, false, nil
	}
	tableFields, tableRegistered := entry.auditFields[normalizeRawSQLIdentifier(db, tableExprFields[0])]
	if !tableRegistered {
		return auditFieldMetadata{}, false, nil
	}
	field, fieldExists := tableFields[fieldName]
	return field, fieldExists, nil
}

// getIsolationTableCacheEntry 返回当前命名策略下缓存的隔离与审计字段元数据。
func getIsolationTableCacheEntry(db *gormdb.DB) isolationTableCacheEntry {
	if db == nil {
		return isolationTableCacheEntry{err: fmt.Errorf("gorm db is nil")}
	}
	models, version := getMigrateModelsSnapshot(db)
	cacheKey := fmt.Sprintf("%p:%T:%#v", getMigrateRegistryIdentity(db), db.NamingStrategy, db.NamingStrategy)
	if cachedValue, cacheFound := isolationTableCache.Load(cacheKey); cacheFound {
		cachedEntry := cachedValue.(isolationTableCacheEntry)
		if cachedEntry.version == version {
			return cachedEntry
		}
	}
	entry := isolationTableCacheEntry{
		version:         version,
		tenantTables:    make(map[string]struct{}),
		dataScopeTables: make(map[string]struct{}),
		auditFields:     make(map[string]map[string]auditFieldMetadata),
	}
	cacheStore := &sync.Map{}
	for _, model := range models {
		modelSchema, err := schema.Parse(model, cacheStore, db.NamingStrategy)
		if err != nil {
			entry.err = err
			break
		}
		if _, hasTenantColumn := modelSchema.FieldsByDBName[tenantColumnName]; hasTenantColumn {
			entry.tenantTables[modelSchema.Table] = struct{}{}
		}
		if _, hasDataScopeColumn := modelSchema.FieldsByDBName[dataScopeCreatedByColumnName]; hasDataScopeColumn {
			entry.dataScopeTables[modelSchema.Table] = struct{}{}
		}
		for _, fieldName := range auditFieldNames {
			field := modelSchema.LookUpField(fieldName)
			if field == nil {
				continue
			}
			fields := entry.auditFields[modelSchema.Table]
			if fields == nil {
				fields = make(map[string]auditFieldMetadata)
				entry.auditFields[modelSchema.Table] = fields
			}
			fields[fieldName] = auditFieldMetadata{name: field.Name, dbName: field.DBName}
		}
	}
	isolationTableCache.Store(cacheKey, entry)
	return entry
}
