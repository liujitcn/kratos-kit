package gorm

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"github.com/liujitcn/kratos-kit/auth/data"
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// callbackRecord 表示同时具有租户、项目和审计字段的业务记录。
type callbackRecord struct {
	ID        int64 `gorm:"primaryKey"`
	TenantID  int64
	ProjectID int64
	Value     int64
	CreatedBy int64
	UpdatedBy int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Creator   *callbackUser `gorm:"foreignKey:CreatedBy;references:ID;belongsTo:Creator"`
}

// TableName 返回回调集成测试的业务表名。
func (callbackRecord) TableName() string { return "callback_records" }

// callbackUser 表示数据范围子查询使用的用户。
type callbackUser struct {
	ID        int64 `gorm:"primaryKey"`
	TenantID  int64
	DeptID    int64
	CreatedBy int64
	Dept      *callbackDept `gorm:"foreignKey:DeptID;references:ID"`
}

// TableName 返回数据范围协议中的用户表名。
func (callbackUser) TableName() string { return "base_user" }

// callbackDept 表示数据范围子查询使用的部门。
type callbackDept struct {
	ID       int64 `gorm:"primaryKey"`
	TenantID int64
	Path     string
}

// TableName 返回数据范围协议中的部门表名。
func (callbackDept) TableName() string { return "base_dept" }

// TestClientPublicCallbackRegistration 验证根包公开注册入口与客户端使用同一个内部注册器。
func TestClientPublicCallbackRegistration(t *testing.T) {
	active := true
	called := 0
	t.Cleanup(func() { active = false })
	RegisterCallbackQuery(func(*gorm.DB) {
		if active {
			called++
		}
	})
	db := newCallbackClient(t)
	called = 0
	var count int64
	err := db.Model(&callbackRecord{}).Count(&count).Error
	if err != nil || called != 1 {
		t.Fatalf("公开注册入口未接入客户端: called=%d err=%v", called, err)
	}
}

// TestClientMigrationKeepsIsolation 验证真实客户端自动迁移后仍执行租户隔离和审计填充。
func TestClientMigrationKeepsIsolation(t *testing.T) {
	db := newCallbackClient(t)
	ctx := callbackContext(1, "0001", DataScopeAll)
	row := callbackRecord{ProjectID: 101}
	err := db.WithContext(ctx).Create(&row).Error
	if err != nil {
		t.Fatal(err)
	}
	if row.TenantID != 1 || row.CreatedBy != 11 || row.UpdatedBy != 11 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		t.Fatalf("迁移后字段填充失效: %+v", row)
	}
	err = db.WithContext(callbackContext(2, "0002", DataScopeAll)).Create(&callbackRecord{ProjectID: 201}).Error
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	err = db.WithContext(ctx).Model(&callbackRecord{}).Count(&count).Error
	if err != nil || count != 1 {
		t.Fatalf("迁移后租户过滤失效: count=%d err=%v", count, err)
	}
}

// TestIsolationBypassIsLocal 验证直接调用、Scopes 和事务中的豁免不污染原会话且不丢失条件。
func TestIsolationBypassIsLocal(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	ctx := callbackContext(1, "0001", DataScopeAll)
	var err error
	for _, viaScope := range []bool{false, true} {
		parent := db.WithContext(ctx).Model(&callbackRecord{}).Where("id = ?", 4)
		var child *gorm.DB
		if viaScope {
			child = parent.Scopes(SkipDataIsolation)
		} else {
			child = SkipDataIsolation(parent)
		}
		var count int64
		err = child.Count(&count).Error
		if err != nil || count != 1 {
			t.Fatalf("豁免丢失条件或未生效: scope=%v count=%d err=%v", viaScope, count, err)
		}
		err = parent.Count(&count).Error
		if err != nil || count != 0 {
			t.Fatalf("豁免污染原查询会话: count=%d err=%v", count, err)
		}
		err = db.WithContext(ctx).Model(&callbackRecord{}).Where("id = ?", 4).Count(&count).Error
		if err != nil || count != 0 {
			t.Fatalf("后续普通查询绕过隔离: count=%d err=%v", count, err)
		}
	}
	rollback := errors.New("回滚测试事务")
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err = SkipDataIsolation(tx).Create(&callbackRecord{ID: 10, TenantID: 2, ProjectID: 201}).Error
		if err != nil {
			return err
		}
		var count int64
		err = tx.Model(&callbackRecord{}).Where("id = ?", 10).Count(&count).Error
		if err != nil || count != 0 {
			t.Fatalf("豁免污染事务: count=%d err=%v", count, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var count int64
	err = db.Model(&callbackRecord{}).Where("id = ?", 10).Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("豁免丢失事务连接: count=%d err=%v", count, err)
	}
}

// TestCallbackDataScopes 验证全部、本人、部门、下级部门及默认租户的范围语义。
func TestCallbackDataScopes(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	cases := []struct {
		name string
		ctx  context.Context
		want int64
	}{
		{"全部", callbackContext(1, "0001", DataScopeAll), 3},
		{"历史未声明", callbackContext(1, "0001", DataScopeUnknown), 3},
		{"本人", callbackContext(1, "0001", DataScopeSelfUser), 1},
		{"本部门", callbackContext(1, "0001", DataScopeSelfDept), 2},
		{"部门及下级", callbackContext(1, "0001", DataScopeDeptAndChildren), 3},
		{"未知限制", callbackContext(1, "0001", 99), 0},
		{"默认租户", callbackContext(1, DefaultTenantCode, DataScopeAll), 4},
		{"无身份系统调用", context.Background(), 4},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var count int64
			err := db.WithContext(item.ctx).Model(&callbackRecord{}).Count(&count).Error
			if err != nil || count != item.want {
				t.Fatalf("count=%d want=%d err=%v", count, item.want, err)
			}
		})
	}
}

// TestCallbackProjectComposition 验证租户、角色范围、项目隔离和审计填充在真实客户端上共同生效。
func TestCallbackProjectComposition(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	err := RegisterProjectIsolation(db, map[string]string{"callback_records": "project_id"}, func(context.Context) (ProjectScope, error) {
		return ProjectScope{Tenants: map[int64][]int64{1: {101}, 2: {201}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := callbackContext(1, "0001", DataScopeSelfUser)
	var rows []callbackRecord
	err = db.WithContext(ctx).Find(&rows).Error
	if err != nil || len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("组合过滤失效: rows=%+v err=%v", rows, err)
	}
	row := callbackRecord{ProjectID: 101}
	err = db.WithContext(ctx).Create(&row).Error
	if err != nil || row.TenantID != 1 || row.CreatedBy != 11 {
		t.Fatalf("租户填充必须先于项目校验: row=%+v err=%v", row, err)
	}
	err = db.WithContext(ctx).Create(&callbackRecord{ProjectID: 102}).Error
	if !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatalf("越权创建未拒绝: %v", err)
	}
	err = db.WithContext(ctx).Model(&callbackRecord{}).Where("id = ?", 1).Update("project_id", 102).Error
	if !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatalf("项目归属修改未拒绝: %v", err)
	}
	result := db.WithContext(ctx).Model(&callbackRecord{}).Where("id = ?", 2).Update("value", 99)
	if result.Error != nil || result.RowsAffected != 0 {
		t.Fatalf("越权更新未过滤: %v", result.Error)
	}
	result = db.WithContext(ctx).Where("id = ?", 4).Delete(&callbackRecord{})
	if result.Error != nil || result.RowsAffected != 0 {
		t.Fatalf("越权删除未过滤: %v", result.Error)
	}
}

// TestCallbackProjectClaims 验证默认项目加载器读取认证范围，缺少身份或范围时不会放大权限。
func TestCallbackProjectClaims(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	err := RegisterProjectIsolation(db, map[string]string{"callback_records": "project_id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	err = db.Model(&callbackRecord{}).Count(&count).Error
	if !errors.Is(err, ErrProjectScopeDenied) {
		t.Fatalf("默认项目加载器未拒绝无身份访问: %v", err)
	}
	err = db.WithContext(callbackContext(1, "0001", DataScopeAll)).Model(&callbackRecord{}).Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("缺少项目范围未收敛为空: count=%d err=%v", count, err)
	}
	identity := &data.UserTokenPayload{TenantId: 1, TenantCode: "0001", UserId: 11, UserName: "callback-test", DataScope: DataScopeAll,
		TenantProjects: []data.TenantProjectScope{{TenantId: 1, ProjectId: []int64{101}}},
	}
	ctx := engine.ContextWithAuthClaims(context.Background(), identity.MakeAuthClaims())
	err = db.WithContext(ctx).Model(&callbackRecord{}).Count(&count).Error
	if err != nil || count != 2 {
		t.Fatalf("认证项目范围未生效: count=%d err=%v", count, err)
	}
}

// TestCallbackRowAndRaw 验证 Row、Rows、Scan 与原生 SQL 的隔离入口。
func TestCallbackRowAndRaw(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	ctx := callbackContext(1, "0001", DataScopeAll)
	var id int64
	err := db.WithContext(ctx).Model(&callbackRecord{}).Select("id").Where("id = ?", 4).Row().Scan(&id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Row 泄漏其他租户记录: %v", err)
	}
	var rows *sql.Rows
	rows, err = db.WithContext(ctx).Model(&callbackRecord{}).Select("id").Rows()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		count++
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}
	err = rows.Close()
	if err != nil || count != 3 {
		t.Fatalf("Rows count=%d err=%v", count, err)
	}
	var result []callbackRecord
	err = db.WithContext(ctx).Model(&callbackRecord{}).Scan(&result).Error
	if err != nil || len(result) != 3 {
		t.Fatalf("Scan len=%d err=%v", len(result), err)
	}
	err = db.WithContext(ctx).Raw("SELECT * FROM callback_records").Scan(&result).Error
	if !errors.Is(err, ErrRawDataIsolationUnsupported) {
		t.Fatalf("Raw 未拒绝: %v", err)
	}
	err = db.WithContext(ctx).Exec("UPDATE callback_records SET value = 99").Error
	if !errors.Is(err, ErrRawDataIsolationUnsupported) {
		t.Fatalf("Exec 未拒绝: %v", err)
	}
	err = db.WithContext(ctx).Raw("SELECT id FROM callback_records").Row().Scan(&id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Raw Row 未拒绝: %v", err)
	}
	err = SkipDataIsolation(db.WithContext(ctx)).Raw("SELECT * FROM callback_records").Scan(&result).Error
	if err != nil || len(result) != 4 {
		t.Fatalf("显式豁免失效: len=%d err=%v", len(result), err)
	}
}

// TestCallbackTenantAndAuditWrites 验证批量、无 Schema map、审计值保留及更新填充。
func TestCallbackTenantAndAuditWrites(t *testing.T) {
	db := newCallbackClient(t)
	ctx := callbackContext(1, "0001", DataScopeAll)
	createdAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []callbackRecord{{ProjectID: 101, CreatedBy: 33, CreatedAt: createdAt}, {ProjectID: 101}}
	err := db.WithContext(ctx).Create(&rows).Error
	if err != nil || rows[0].CreatedBy != 33 || !rows[0].CreatedAt.Equal(createdAt) || rows[1].TenantID != 1 {
		t.Fatalf("批量审计或租户填充异常: rows=%+v err=%v", rows, err)
	}
	values := map[string]interface{}{"project_id": int64(101), "value": int64(42)}
	err = db.WithContext(ctx).Table("callback_records").Create(values).Error
	if err != nil || !reflect.DeepEqual(values["tenant_id"], int64(1)) || values["created_by"] != int64(11) {
		t.Fatalf("无 Schema map 填充异常: values=%v err=%v", values, err)
	}
	err = db.WithContext(ctx).Create(&callbackRecord{TenantID: 2}).Error
	if err == nil {
		t.Fatalf("跨租户创建未拒绝: %v", err)
	}
	err = db.WithContext(ctx).Model(&callbackRecord{}).Where("id = ?", rows[0].ID).Updates(map[string]interface{}{"value": 99, "updated_by": 33, "updated_at": createdAt}).Error
	if err != nil {
		t.Fatal(err)
	}
	var updated callbackRecord
	err = db.WithContext(ctx).First(&updated, rows[0].ID).Error
	if err != nil || updated.UpdatedBy != 11 || !updated.UpdatedAt.After(createdAt) || updated.CreatedBy != 33 {
		t.Fatalf("更新审计异常: row=%+v err=%v", updated, err)
	}
}

// newCallbackClient 通过生产构造入口创建启用自动迁移的独立内存数据库。
func newCallbackClient(t *testing.T) *gorm.DB {
	t.Helper()
	const driverName = "callback-test-sqlite"
	previous, existed := driver.Opens[driverName]
	driver.Opens[driverName] = sqlite.Open
	t.Cleanup(func() {
		if existed {
			driver.Opens[driverName] = previous
		} else {
			delete(driver.Opens, driverName)
		}
	})
	client, cleanup, err := NewGormClient(&configv1.Data_Database{Driver: driverName, Source: ":memory:", EnableMigrate: true}, WithMigrateModels(&callbackRecord{}, &callbackUser{}, &callbackDept{}))
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return client.DB
}

// callbackContext 构建固定用户和部门的认证上下文。
func callbackContext(tenantID int64, tenantCode string, scope int32) context.Context {
	identity := &data.UserTokenPayload{TenantId: tenantID, TenantCode: tenantCode, UserId: 11, UserName: "callback-test", DeptId: 10, DataScope: scope}
	return engine.ContextWithAuthClaims(context.Background(), identity.MakeAuthClaims())
}

// seedCallbackRecords 构造跨租户、跨部门和跨项目数据，供隔离行为测试复用。
func seedCallbackRecords(t *testing.T, db *gorm.DB) {
	t.Helper()
	fixtures := []interface{}{
		&[]callbackDept{{ID: 10, TenantID: 1, Path: "/0/"}, {ID: 20, TenantID: 1, Path: "/0/10/"}, {ID: 30, TenantID: 2, Path: "/0/"}},
		&[]callbackUser{{ID: 11, TenantID: 1, DeptID: 10, CreatedBy: 11}, {ID: 12, TenantID: 1, DeptID: 10, CreatedBy: 11}, {ID: 13, TenantID: 1, DeptID: 20, CreatedBy: 12}, {ID: 21, TenantID: 2, DeptID: 30, CreatedBy: 21}},
		&[]callbackRecord{{ID: 1, TenantID: 1, ProjectID: 101, CreatedBy: 11}, {ID: 2, TenantID: 1, ProjectID: 101, CreatedBy: 12}, {ID: 3, TenantID: 1, ProjectID: 102, CreatedBy: 13}, {ID: 4, TenantID: 2, ProjectID: 201, CreatedBy: 21}},
	}
	for _, fixture := range fixtures {
		err := db.WithContext(context.Background()).Create(fixture).Error
		if err != nil {
			t.Fatal(err)
		}
	}
}
