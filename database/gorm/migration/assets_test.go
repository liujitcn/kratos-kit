package migration

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

// TestLoadMigrationAssetsSkipsUnknownDatabaseTypes 验证未知数据库类型目录被跳过告警而不阻断加载。
func TestLoadMigrationAssetsSkipsUnknownDatabaseTypes(t *testing.T) {
	assetsFS := fstest.MapFS{
		"v0.0.1/mysql/base_init.up.sql":    &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
		"v0.0.1/postgres/base_init.up.sql": &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
		"v0.0.1/dameng/base_init.up.sql":   &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
	}
	assets, err := loadMigrationAssets(assetsFS, ".")
	if err != nil {
		t.Fatalf("未知类型目录不应阻断加载: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("期望加载 mysql、postgres 两个资产实际 %d：%v", len(assets), assets)
	}
	for _, asset := range assets {
		if asset.databaseType != databaseTypeMySQL && asset.databaseType != databaseTypePostgres {
			t.Fatalf("不应加载未知数据库类型资产: %s", asset.databaseType)
		}
		if asset.versionName != "v0.0.1" || asset.dataSource != DefaultTarget {
			t.Fatalf("资产版本或数据源解析错误: %s %s", asset.versionName, asset.dataSource)
		}
	}
}

// TestLoadMigrationAssetsRejectsLooseFiles 验证版本目录下的非说明直系文件仍然报错。
func TestLoadMigrationAssetsRejectsLooseFiles(t *testing.T) {
	assetsFS := fstest.MapFS{
		"v0.0.1/loose.sql": &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
	}
	if _, err := loadMigrationAssets(assetsFS, "."); err == nil {
		t.Fatal("版本目录直系文件应报错")
	}
}

// TestLoadMigrationAssetsSharesVersionDescriptions 验证版本级公用 md 挂到全部数据库类型资产的描述引用。
func TestLoadMigrationAssetsSharesVersionDescriptions(t *testing.T) {
	assetsFS := fstest.MapFS{
		"v0.0.1/README.md":                 &fstest.MapFile{Data: []byte("# 初始化说明\n")},
		"v0.0.1/mysql/base_init.up.sql":    &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
		"v0.0.1/postgres/base_init.up.sql": &fstest.MapFile{Data: []byte("INSERT INTO t VALUES (1);")},
	}
	assets, err := loadMigrationAssets(assetsFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("期望两个资产实际 %d", len(assets))
	}
	for _, asset := range assets {
		var references []FileReference
		if err = json.Unmarshal([]byte(asset.description), &references); err != nil {
			t.Fatalf("描述引用解析失败: %v", err)
		}
		if len(references) != 1 || references[0].Path != "v0.0.1/README.md" {
			t.Fatalf("资产 %s 的描述引用应包含版本级公用 md：%s", asset.databaseType, asset.description)
		}
	}
}
