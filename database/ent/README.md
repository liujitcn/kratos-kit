# database/ent

`database/ent` 封装 Ent 使用的底层 `dialect.Driver`，用于复用 `kratos-kit` 中统一的数据库配置、连接池、debug SQL 日志、迁移回调、表注释和审计字段能力。

## 安装

按实际数据库导入对应 driver 子模块：

```bash
go get github.com/liujitcn/kratos-kit/database/ent@latest
go get github.com/liujitcn/kratos-kit/database/ent/driver/mysql@latest
go get github.com/liujitcn/kratos-kit/database/ent/driver/postgres@latest
go get github.com/liujitcn/kratos-kit/database/ent/driver/sqlite@latest
```

## 使用

Ent 的 `*ent.Client` 是业务项目按 schema 生成的类型，本模块只返回通用 driver：

```go
import (
	"context"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	entkit "github.com/liujitcn/kratos-kit/database/ent"
	_ "github.com/liujitcn/kratos-kit/database/ent/driver/mysql"

	"your-app/ent"
)

func NewClient(cfg *configv1.Data_Database) (*ent.Client, func(), error) {
	drv, cleanup, err := entkit.NewEntClient(cfg)
	if err != nil {
		return nil, nil, err
	}

	client := ent.NewClient(ent.Driver(drv))
	if cfg.GetEnableMigrate() {
		if err = client.Schema.Create(context.Background()); err != nil {
			cleanup()
			return nil, nil, err
		}
		if err = drv.RunRegisteredTableComments(context.Background()); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	return client, cleanup, nil
}
```

## 迁移回调

若希望由 `NewEntClient` 在 `enable_migrate` 开启时统一触发迁移，可注册迁移函数；注册迁移执行成功后，本模块会自动执行已注册的表注释回填：

```go
import "entgo.io/ent/dialect"

entkit.RegisterMigrate(func(ctx context.Context, drv dialect.Driver) error {
	client := ent.NewClient(ent.Driver(drv))
	return client.Schema.Create(ctx)
})
```

## 审计字段

在 Ent schema 中加入 `AuditMixin` 后，会自动提供并回填 `created_by`、`updated_by`、`created_at`、`updated_at`：

```go
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entkit.AuditMixin{},
	}
}
```

## 表注释

字段注释通过 Ent 原生 `field.Comment(...)` 和 `entsql.WithComments(true)` 落库。表注释可在迁移前注册：

```go
entkit.RegisterTableComment("users", "用户表")
```

当前内置 driver 支持 `mysql`、`doris`、`postgres`/`postgresql`、`sqlite`/`sqlite3`。`doris` 复用 MySQL 协议连接能力，导入 `database/ent/driver/mysql` 即可注册。`enable_trace` 与 `enable_metrics` 会在 Ent `dialect.Driver` 层记录 SQL 操作追踪、操作次数和耗时指标。

Doris 建表、分区、分桶等能力建议使用专用 SQL 管理，不建议依赖 Ent 自动迁移。
