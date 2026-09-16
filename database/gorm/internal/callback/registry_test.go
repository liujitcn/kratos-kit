package callback

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"

	"gorm.io/gorm"
)

// TestCustomCallbackLifecycle 验证全部公开扩展入口、批量空项和前后置时机仍然可用。
func TestCustomCallbackLifecycle(t *testing.T) {
	preserveCustomCallbacks(t)
	var events []string
	record := func(name string) func(*gorm.DB) {
		return func(*gorm.DB) { events = append(events, name) }
	}
	RegisterCallbackQuery(record("query"))
	RegisterCallbackQueries(nil, record("queries"))
	RegisterCallbackQueryAfter(record("query-after"))
	RegisterCallbackRow(record("row"))
	RegisterCallbackRaw(record("raw"))
	RegisterCallbackCreate(record("create"))
	RegisterCallbackCreates(nil, record("creates"))
	RegisterCallbackCreateAfter(record("create-after"))
	RegisterCallbackUpdate(record("update"))
	RegisterCallbackUpdates(nil, record("updates"))
	RegisterCallbackUpdateBefore("", record("update-default-anchor"))
	RegisterCallbackUpdateBefore("gorm:update", record("update-sql"))
	RegisterCallbackUpdateAfter(record("update-after"))
	RegisterCallbackDelete(record("delete"))
	RegisterCallbackDeletes(nil, record("deletes"))
	RegisterCallbackDeleteAfter(record("delete-after"))
	db := newRegistryTestDB(t)
	steps := []struct {
		name string
		run  func() error
		want []string
	}{
		{"create", func() error { return db.Create(&registryRecord{ID: 1}).Error }, []string{"create", "creates", "create-after"}},
		{"query", func() error {
			var rows []registryRecord
			return db.Find(&rows).Error
		}, []string{"query", "queries", "query-after"}},
		{"row", func() error {
			var id int64
			return db.Model(&registryRecord{}).Select("id").Row().Scan(&id)
		}, []string{"row"}},
		{"update", func() error {
			return db.Model(&registryRecord{}).Where("id = ?", 1).Update("value", 10).Error
		}, []string{"update", "updates", "update-default-anchor", "update-sql", "update-after"}},
		{"raw", func() error { return db.Exec("UPDATE registry_records SET value = 11 WHERE id = 1").Error }, []string{"raw"}},
		{"delete", func() error { return db.Delete(&registryRecord{ID: 1}).Error }, []string{"delete", "deletes", "delete-after"}},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			events = nil
			err := step.run()
			if err != nil || !reflect.DeepEqual(events, step.want) {
				t.Fatalf("events=%v want=%v err=%v", events, step.want, err)
			}
		})
	}
}

// TestCustomCallbackSnapshot 验证创建后的客户端不受晚注册项影响，新客户端加载完整快照。
func TestCustomCallbackSnapshot(t *testing.T) {
	preserveCustomCallbacks(t)
	db := newRegistryTestDB(t)
	called := 0
	RegisterCallbackQuery(func(*gorm.DB) { called++ })
	var count int64
	err := db.Model(&registryRecord{}).Count(&count).Error
	if err != nil || called != 0 {
		t.Fatalf("旧客户端被晚注册项改变: called=%d err=%v", called, err)
	}
	other := newRegistryTestDB(t)
	called = 0
	err = other.Model(&registryRecord{}).Count(&count).Error
	if err != nil || called != 1 {
		t.Fatalf("新客户端未加载扩展: called=%d err=%v", called, err)
	}
}

// TestCustomAfterCallbacksRollback 验证创建、更新和删除的后置回调都在事务提交前执行。
func TestCustomAfterCallbacksRollback(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			preserveCustomCallbacks(t)
			rejected := errors.New("后置校验拒绝")
			armed := false
			handler := func(db *gorm.DB) {
				if armed {
					db.AddError(rejected)
				}
			}
			switch operation {
			case "create":
				RegisterCallbackCreateAfter(handler)
			case "update":
				RegisterCallbackUpdateAfter(handler)
			case "delete":
				RegisterCallbackDeleteAfter(handler)
			}
			db := newRegistryTestDB(t)
			err := db.Create(&registryRecord{ID: 1, Value: 10}).Error
			if err != nil {
				t.Fatal(err)
			}
			armed = true
			switch operation {
			case "create":
				err = db.Create(&registryRecord{ID: 2}).Error
			case "update":
				err = db.Model(&registryRecord{}).Where("id = ?", 1).Update("value", 99).Error
			case "delete":
				err = db.Delete(&registryRecord{ID: 1}).Error
			}
			if !errors.Is(err, rejected) {
				t.Fatalf("未保留后置错误: %v", err)
			}
			armed = false
			var rows []registryRecord
			err = db.Find(&rows).Error
			if err != nil || len(rows) != 1 || rows[0].Value != 10 {
				t.Fatalf("后置错误未回滚: rows=%+v err=%v", rows, err)
			}
		})
	}
}

// preserveCustomCallbacks 隔离测试使用的包级扩展注册状态。
func preserveCustomCallbacks(t *testing.T) {
	t.Helper()
	snapshot := customCallbackSnapshot()
	t.Cleanup(func() {
		registeredCallbackMu.Lock()
		defer registeredCallbackMu.Unlock()
		customCallbacks = snapshot
	})
}

// registryRecord 表示扩展回调测试使用的业务记录。
type registryRecord struct {
	ID    int64 `gorm:"primaryKey"`
	Value int64
}

// TableName 返回扩展回调测试表名。
func (registryRecord) TableName() string { return "registry_records" }

// newRegistryTestDB 安装当前回调快照并创建独立内存数据库。
func newRegistryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var connection *sql.DB
	connection, err = db.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() {
		closeErr := connection.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})
	db = BindModels(db, []interface{}{&registryRecord{}}, true)
	err = Install(db)
	if err != nil {
		t.Fatal(err)
	}
	err = SkipDataIsolation(db).AutoMigrate(&registryRecord{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
