package gorm

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// projectIsolationRow 表示项目隔离测试使用的业务记录。
type projectIsolationRow struct {
	ID        int64 `gorm:"primaryKey"`
	TenantID  int64
	ProjectID int64
	Value     int64
}

// TableName 返回测试业务表名。
func (projectIsolationRow) TableName() string { return "project_records" }

// newProjectIsolationTestDB 创建包含多个租户项目的独立测试库。
func newProjectIsolationTestDB(t *testing.T, scope ProjectScope, loadErr error) *gorm.DB {
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
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	err = db.AutoMigrate(&projectIsolationRow{})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Create(&[]projectIsolationRow{{ID: 1, TenantID: 1, ProjectID: 101, Value: 10}, {ID: 2, TenantID: 1, ProjectID: 102, Value: 20}, {ID: 3, TenantID: 2, ProjectID: 101, Value: 30}, {ID: 4, TenantID: 2, ProjectID: 201, Value: 40}}).Error
	if err != nil {
		t.Fatal(err)
	}
	err = RegisterProjectIsolation(db, map[string]string{"project_records": "project_id"}, func(context.Context) (ProjectScope, error) { return scope, loadErr })
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// TestProjectIsolationQueries 验证跨租户配对、全部、空权限、分页和计数。
func TestProjectIsolationQueries(t *testing.T) {
	cases := []struct {
		name  string
		scope ProjectScope
		want  int64
	}{
		{"指定配对", ProjectScope{Tenants: map[int64][]int64{1: {101}, 2: {201}}}, 2},
		{"单租户全部", ProjectScope{Tenants: map[int64][]int64{1: {0}}}, 2},
		{"空权限", ProjectScope{Tenants: map[int64][]int64{}}, 0},
		{"系统任务", ProjectScope{System: true}, 4},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			db := newProjectIsolationTestDB(t, item.scope, nil)
			var count int64
			err := db.Model(&projectIsolationRow{}).Count(&count).Error
			if err != nil || count != item.want {
				t.Fatalf("count=%d err=%v", count, err)
			}
			var rows []projectIsolationRow
			err = db.Order("id").Limit(1).Find(&rows).Error
			if err != nil {
				t.Fatal(err)
			}
			if item.want == 0 && len(rows) != 0 {
				t.Fatal("空权限泄漏记录")
			}
		})
	}
}

// TestProjectIsolationWrites 验证新增、修改和删除均受项目范围约束。
func TestProjectIsolationWrites(t *testing.T) {
	db := newProjectIsolationTestDB(t, ProjectScope{Tenants: map[int64][]int64{1: {101}}}, nil)
	err := db.Create(&projectIsolationRow{ID: 5, TenantID: 1, ProjectID: 102}).Error
	if !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatalf("越权新增未拒绝: %v", err)
	}
	err = db.Create(&projectIsolationRow{ID: 6, TenantID: 1, ProjectID: 101}).Error
	if err != nil {
		t.Fatal(err)
	}
	changed := db.Model(&projectIsolationRow{}).Where("id = ?", 2).Update("value", 99)
	if changed.Error != nil || changed.RowsAffected != 0 {
		t.Fatalf("越权更新: %v", changed.Error)
	}
	err = db.Model(&projectIsolationRow{}).Where("id = ?", 1).Update("project_id", 102).Error
	if !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatal("项目归属迁移未拒绝")
	}
	deleted := db.Where("id = ?", 3).Delete(&projectIsolationRow{})
	if deleted.Error != nil || deleted.RowsAffected != 0 {
		t.Fatalf("越权删除: %v", deleted.Error)
	}
}

// TestProjectIsolationFailure 验证加载失败、非法范围及原生 SQL 不会退化为全库查询。
func TestProjectIsolationFailure(t *testing.T) {
	denied := errors.New("授权服务不可用")
	db := newProjectIsolationTestDB(t, ProjectScope{}, denied)
	var rows []projectIsolationRow
	if err := db.Find(&rows).Error; !errors.Is(err, denied) {
		t.Fatalf("错误未保留: %v", err)
	}
	db = newProjectIsolationTestDB(t, ProjectScope{Tenants: map[int64][]int64{1: {0, 101}}}, nil)
	if err := db.Find(&rows).Error; !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatal("非法范围未拒绝")
	}
	db = newProjectIsolationTestDB(t, ProjectScope{Tenants: map[int64][]int64{1: {101}}}, nil)
	if err := db.Raw("SELECT * FROM project_records").Scan(&rows).Error; err == nil {
		t.Fatal("原生SQL绕过隔离")
	}
}

// TestProjectIsolationJoin 验证别名和左连接两侧分别应用配对范围。
func TestProjectIsolationJoin(t *testing.T) {
	db := newProjectIsolationTestDB(t, ProjectScope{Tenants: map[int64][]int64{1: {101}, 2: {201}}}, nil)
	var rows []struct {
		ID       int64
		JoinedID *int64
	}
	err := db.Model(&projectIsolationRow{}).Table("project_records AS root").Select("root.id, linked.id AS joined_id").Joins("LEFT JOIN project_records AS linked ON linked.project_id = root.project_id AND linked.id != root.id").Order("root.id").Scan(&rows).Error
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[0].JoinedID != nil {
		t.Fatalf("左连接泄漏未授权关联: %+v", rows)
	}
}
