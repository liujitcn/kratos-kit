package gorm

import (
	"errors"
	"testing"
)

// TestIsolationJoinComposition 验证 JOIN 改写同时保留主表、关联表的租户和角色范围。
func TestIsolationJoinComposition(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	ctx := callbackContext(1, "0001", DataScopeSelfUser)
	var err error
	for _, join := range []string{
		"LEFT JOIN callback_records AS linked ON linked.id = 4",
		"LEFT JOIN callback_records AS linked ON linked.id = 4 OR linked.id = 2",
		"LEFT JOIN callback_records AS linked ON (linked.id = 4 OR linked.id = 2) /* JOIN ignored */",
	} {
		var rows []struct {
			ID       int64
			LinkedID *int64
		}
		err = db.WithContext(ctx).Model(&callbackRecord{}).Table("callback_records AS root").Select("root.id, linked.id AS linked_id").Joins(join).Scan(&rows).Error
		if err != nil || len(rows) != 1 || rows[0].ID != 1 || rows[0].LinkedID != nil {
			t.Fatalf("LEFT JOIN 语义或隔离改变: join=%s rows=%+v err=%v", join, rows, err)
		}
	}
	for _, join := range []string{
		"INNER JOIN callback_records AS linked ON linked.id = root.id",
		"INNER JOIN callback_records AS linked USING (id)",
		"CROSS JOIN callback_records AS linked",
	} {
		var count int64
		err = db.WithContext(ctx).Model(&callbackRecord{}).Table("callback_records AS root").Joins(join).Count(&count).Error
		if err != nil || count != 1 {
			t.Fatalf("JOIN 过滤改变: join=%s count=%d err=%v", join, count, err)
		}
	}
}

// TestIsolationNestedAssociations 验证共享 JOIN 逻辑仍能展开并过滤嵌套模型关联。
func TestIsolationNestedAssociations(t *testing.T) {
	db := newCallbackClient(t)
	seedCallbackRecords(t, db)
	var rows []callbackRecord
	err := db.WithContext(callbackContext(1, "0001", DataScopeSelfUser)).Joins("Creator.Dept").Find(&rows).Error
	if err != nil || len(rows) != 1 || rows[0].Creator == nil || rows[0].Creator.Dept == nil || rows[0].Creator.Dept.ID != 10 {
		t.Fatalf("嵌套模型关联异常: rows=%+v err=%v", rows, err)
	}
}

// TestIsolationUnsupportedJoins 验证无法安全改写的关联仍然拒绝执行。
func TestIsolationUnsupportedJoins(t *testing.T) {
	db := newCallbackClient(t)
	ctx := callbackContext(1, "0001", DataScopeAll)
	for _, join := range []string{
		"LEFT JOIN callback_records AS linked USING (id)",
		"NATURAL LEFT JOIN callback_records AS linked",
		"FULL OUTER JOIN callback_records AS linked ON linked.id = callback_records.id",
		"LEFT JOIN (SELECT * FROM callback_records) AS linked ON linked.id = callback_records.id",
	} {
		var rows []callbackRecord
		err := db.WithContext(ctx).Joins(join).Find(&rows).Error
		if !errors.Is(err, ErrRawDataIsolationUnsupported) {
			t.Fatalf("不支持的关联未拒绝: join=%s err=%v", join, err)
		}
	}
}
