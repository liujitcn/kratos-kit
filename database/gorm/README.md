# database/gorm

`database/gorm` 是基于 GORM 的数据库客户端封装，统一提供驱动注册、连接池、自动迁移、可观测性、审计字段填充、租户隔离和角色数据范围过滤。

## 能力概览

| 能力 | 当前行为 |
| --- | --- |
| 数据库驱动 | 支持 `mysql`、`doris`、`postgres`、`sqlite`、`sqlserver`、`oracle`、`bigquery`，使用前需要导入对应 driver 子模块 |
| 连接管理 | 支持最大空闲连接数、最大打开连接数和连接最大生命周期 |
| SQL 日志 | `debug: true` 时输出 GORM Info 级别 SQL 日志，慢 SQL 阈值为 1 秒 |
| 链路追踪 | `enable_trace: true` 时启用 GORM OpenTelemetry tracing 插件 |
| Prometheus | `enable_metrics: true` 时启用 GORM Prometheus 插件 |
| 自动迁移 | `enable_migrate: true` 时对已注册模型执行 `AutoMigrate` |
| 表注释 | 自动迁移后为实现 `TableCommenter` 的已注册模型回填表注释 |
| 审计字段 | 创建时填充 `created_by`、`updated_by`、`created_at`、`updated_at`；更新时刷新 `updated_by`、`updated_at` |
| 租户隔离 | 自动填充 `tenant_id`，并为查询、更新、删除和关联查询追加租户条件 |
| 数据范围 | 根据登录 token 中的数据范围过滤本人、本部门、本部门及下级部门数据 |
| 安全边界 | 受保护数据缺少身份时默认拒绝；无法安全改写的 Raw SQL 或复杂 JOIN 默认拒绝 |
| 扩展回调 | 支持注册 Query、Row、Raw、Create、Update、Delete 回调 |

## 安装

安装 GORM 封装和实际使用的 driver 子模块：

```bash
go get github.com/liujitcn/kratos-kit/database/gorm@latest
go get github.com/liujitcn/kratos-kit/database/gorm/driver/mysql@latest
go get github.com/liujitcn/kratos-kit/database/gorm/migration@latest
```

可用 driver 导入路径：

```go
import (
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/bigquery"
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/mysql" // 同时注册 mysql、doris
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/oracle"
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/postgres"
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/sqlite"
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/sqlserver"
)
```

业务项目只需导入实际使用的 driver，不要同时导入全部实现。

## 快速接入

### 1. 定义模型

模型可以通过 `WithMigrateModels` 绑定到单个客户端；不传显式模型时仍兼容使用包级注册表。模型信息同时用于自动迁移、表注释、无 Schema map 填充以及关联表隔离识别。

```go
package data

import (
	"time"
)

type Test struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TenantID  int64     `gorm:"column:tenant_id;not null;index"`
	CreatedBy int64     `gorm:"column:created_by;not null;index"`
	UpdatedBy int64     `gorm:"column:updated_by;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName 返回订单表名。
func (*Test) TableName() string {
	return "test"
}

// TableComment 返回订单表注释。
func (*Test) TableComment() string {
	return "测试"
}

```

### 2. 创建客户端

```go
package data

import (
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	gormkit "github.com/liujitcn/kratos-kit/database/gorm"
	_ "github.com/liujitcn/kratos-kit/database/gorm/driver/mysql"
)

func NewDB(cfg *configv1.Data_Database) (*gormkit.Client, func(), error) {
	return gormkit.NewGormClient(
		cfg,
		gormkit.WithMigrateModels(&Order{}),
	)
}
```

如果项目仍使用包级注册，也可以在创建客户端前调用 `RegisterMigrateModel` 或 `RegisterMigrateModels`；只有未传 `WithMigrateModels` 时才会回退到这份全局注册表。

配置对应 `config.v1.Data.Database`：

```yaml
driver: mysql
source: "root:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=true&loc=Local"
debug: false
enable_migrate: true
enable_trace: true
enable_metrics: false
max_idle_connections: 10
max_open_connections: 100
connection_max_lifetime: 1h
```

`NewGormClient` 返回的 `cleanup` 用于关闭底层 `sql.DB`。

## 多数据源

`Data` 同时支持旧的单数据库配置和按名称区分的多个数据库配置：

```yaml
data:
  databases:
    main:
      driver: mysql
      source: "root:password@tcp(127.0.0.1:3306)/test"
      connection_timeout: 5s
    audit:
      driver: postgres
      source: "postgres://user:password@127.0.0.1:5432/audit"
      prometheus_http_port: 8081
```

旧的 `data.database` 会按名称 `default` 兼容；如果同时存在 `database` 与 `databases`，两者合并，重复的 `default` 直接报错。业务代码应为每个数据源显式注入独立的 `data.Data`，不在请求中动态切库，也不跨数据源执行事务或 Join。

每个客户端创建时会主动 Ping 数据库，连接校验默认超时为 `5s`，可通过 `connection_timeout` 覆盖。显式模型范围优先于旧的全局注册表：

```go
client, cleanup, err := gormkit.NewGormClient(
    cfg,
    gormkit.WithName("audit"),
    gormkit.WithMigrateModels(auditdata.Models()...),
)
```

启用 metrics 的多个客户端必须使用不同的 `prometheus_http_port`；未显式配置 `prometheus_db_name` 时使用客户端名称。

## 版本化迁移

`database/gorm/migration` 提供与 admin 无关的版本化迁移能力。业务模块通过
`embed.FS` 提供按版本目录组织的迁移资源，使用 `Name` 标识模块、`Target` 选择
已注入的 GORM 客户端，并可通过 `Dependencies` 声明执行顺序。
所有模块统一使用默认 `data.database` 中的 `base_migration` 表记录版本，使用
`business` 字段区分模块，并保存 `description` 描述。资源目录最外层只放纯数字
版本目录支持兼容的纯数字格式（如 `000001`）以及 `v0.0.1`、
`v0.0.1-20260511170946`、`v0.0.1.20260511170946` 三种版本格式。目录中的
`*.up.sql`、`*.down.sql` 和 `.md` 文件均为可选；所有 `.md` 文件
按文件名排序后直接拼接为版本描述。`*.down.sql` 文件只保存，不参与当前升级执行。多个升级
脚本按文件名排序，在同一个数据库事务中执行，任一脚本失败时回滚该版本的脚本。目录可以
只有描述、只有脚本或完全为空；没有升级脚本时仍记录并标记该版本成功。脚本仍连接各自
`Target` 数据源执行，只有版本记录集中保存到默认数据库。

`base_migration` 对应的 `BaseMigration` 模型位于 `database/gorm/migration`，迁移包
只负责使用，不会自动注册或建表；宿主项目（例如 kratos-admin）在 GORM 建表前注册该模型。
默认 GORM 客户端在 `enable_migrate: true` 时通过 `AutoMigrate` 创建。迁移执行器不会再自行创建这张表；
默认中心数据库客户端未注入或未开启 `enable_migrate` 时，所有版本化脚本都会跳过，
目标客户端未注入或未开启时也会跳过该目标的脚本。接入项目不需要提前手工建表。
`base_migration.up_sql` 保存本次版本实际执行的全部升级脚本，`down_sql` 预留给后续回退能力，
当前不会执行回退脚本。升级脚本执行失败会保留失败记录，应用继续启动，并在后续启动时
重试该版本。

```go
// Migrations 返回业务模块的版本化迁移资源。
func (contributor) Migrations() []migration.MigrationSpec {
	return []migration.MigrationSpec{{
		Name: "test", // 写入 base_migration.business。
		FS:   migrationFS,
		Path: "assets/mysql",
	}}
}

registry, err := migration.NewRegistry(
    migration.AdditionalMigrations{contributor{}},
)
runner, err := migration.NewRunner(registry)
// client 为宿主创建的默认 *gorm.Client。
err = runner.SetClient(client)
// shopClient 为可选的、名称为 shop 的其他数据源客户端。
if shopClient != nil {
    err = runner.SetClient(shopClient)
}
err = runner.Run(ctx, "test", migration.DefaultTarget)
```

迁移核心通过客户端名称选择脚本执行目标。名称为 `default` 的客户端使用 GORM
保存和查询 `base_migration` 记录；其他名称的客户端只执行自身数据源脚本，不在
自身数据库写入迁移记录。客户端的 `enable_migrate` 为 `false` 时跳过对应脚本，
不要求接入项目依赖 kratos-admin 或 Wire。

## 自动填充的数据

### 创建数据

模型声明对应字段时，Create 回调按下表填充；用户字段要求 context 中存在认证声明，时间字段不要求 Token：

| Go 字段 | 数据库列 | 填充值 | 覆盖规则 |
| --- | --- | --- | --- |
| `TenantID` | `tenant_id` | 当前登录用户的租户 ID | 空值时填充；普通租户显式传入其他租户 ID 时拒绝创建 |
| `CreatedBy` | `created_by` | 当前登录用户 ID | 仅空值时填充 |
| `UpdatedBy` | `updated_by` | 当前登录用户 ID | 仅空值时填充 |
| `CreatedAt` | `created_at` | 当前时间 | 仅空值时填充 |
| `UpdatedAt` | `updated_at` | 当前时间 | 仅空值时填充 |

调用方显式设置的非零创建审计值不会被覆盖。批量结构体、`map[string]interface{}` 和 `[]map[string]interface{}` 均支持填充。

使用 `Table(...).Create(map)` 且没有 GORM Schema 时，目标表必须包含在当前客户端的 `WithMigrateModels` 模型中，或在未使用显式模型时通过 `RegisterMigrateModel`、`RegisterMigrateModels` 注册，回调才会确认字段真实存在并进行填充。

### 更新数据

模型声明对应字段时，Update 回调按下表刷新；用户字段要求 context 中存在认证声明，时间字段不要求 Token：

| Go 字段 | 数据库列 | 填充值 | 覆盖规则 |
| --- | --- | --- | --- |
| `UpdatedBy` | `updated_by` | 当前登录用户 ID | 每次更新都刷新 |
| `UpdatedAt` | `updated_at` | 当前时间 | 每次更新都刷新 |

更新不会修改 `CreatedBy` 和 `CreatedAt`。无 Schema map 更新同样要求目标表已经注册。

### 用户信息来源

用户、部门、租户和数据范围均从 `auth.FromContext` 解析，正常请求应使用认证中间件生成的 context。

- 普通请求：使用 token 中的 `UserId`、`DeptId`、`TenantId`、`TenantCode`、`DataScope`。
- 未携带 token 或 context 中没有认证声明：租户隔离、角色数据范围和 Raw SQL 拒绝回调全部跳过；审计回调只填充时间字段。
- `context.Background()`、`context.TODO()` 或 Kratos 应用生命周期 context：不填充用户审计字段，时间审计字段仍正常填充。
- `base_log`：完全跳过审计字段填充。

## 租户隔离

只要当前模型的数据库列包含 `tenant_id`，或者目标表已注册为租户表，就会启用租户能力。

### 普通租户

- 创建：空 `tenant_id` 自动填入当前租户 ID。
- 创建：显式租户 ID 与当前租户不一致时返回错误。
- 查询、更新、删除：自动追加当前租户条件。
- JOIN：关联到租户表时，为关联表追加租户条件。
- 未携带 token：跳过租户填充与隔离，不追加租户条件。
- 已携带 token 但缺少有效租户 ID：受保护操作返回 `ErrTenantContextMissing`。

### 默认租户

`TenantCode == "0000"` 的默认租户拥有跨租户访问能力，查询、更新和删除不追加租户条件。

默认租户创建数据时不会自动推断目标租户，调用方应明确提供 `tenant_id`。默认租户能力不等同于 Raw SQL 豁免。

## 角色数据范围

数据范围应用于以下对象：

- GORM Schema 包含 `created_by` 的模型。
- 已注册且包含 `created_by` 的表，包括无 Schema map 查询。
- 固定用户表 `base_user`。
- 固定部门表 `base_dept`。

特殊表名和列名属于当前数据范围协议：

| 表 | 必需列 | 用途 |
| --- | --- | --- |
| `base_user` | `id`、`tenant_id`、`dept_id`、`created_by` | 根据创建人查找所属部门，并对用户数据本身按创建人过滤 |
| `base_dept` | `id`、`tenant_id`、`path` | 判断当前部门和下级部门范围 |
| 其他受保护表 | `created_by`；多租户表还需 `tenant_id` | 关联创建人并叠加租户条件 |

当前数据范围常量：

| 常量 | 值 | 行为 |
| --- | ---: | --- |
| `DataScopeUnknown` | 0 | 未声明数据范围，为兼容历史 token 不追加数据范围条件 |
| `DataScopeAll` | 1 | 全部数据，不追加数据范围条件；租户隔离仍然生效 |
| `DataScopeDeptAndChildren` | 2 | 数据创建人属于当前部门或下级部门 |
| `DataScopeSelfDept` | 3 | 数据创建人属于当前部门 |
| `DataScopeSelfUser` | 4 | `created_by` 等于当前用户 ID |

“本人数据”表示当前用户创建的数据，不表示记录主键等于当前用户 ID。该语义同样适用于 `base_user`。

### 普通业务表和 base_user

- 本人：`created_by = 当前用户 ID`。
- 本部门：通过 `base_user` 判断创建人当前所属部门。
- 本部门及下级：通过 `base_user JOIN base_dept` 判断创建人所在部门是否属于当前部门树。
- `base_user` 的部门范围使用派生创建人集合，避免 MySQL 更新或删除目标表时触发 1093 错误。

### base_dept

部门表不按创建人过滤，而是直接按部门范围过滤：

- 本人、本部门：只访问当前部门 ID。
- 本部门及下级：当前部门 ID，或者 `path LIKE '%/{当前部门ID}/%'`。

`base_dept.path` 应使用 `/` 分隔祖先部门 ID，例如 `/0/1/2`。

未携带 token 时跳过角色数据范围条件。未知的非零限制型数据范围按无权限处理。

## JOIN 处理

回调支持 GORM 模型关联、嵌套关联和可安全识别的原生 JOIN。

| JOIN 形式 | 隔离条件位置或行为 |
| --- | --- |
| GORM `Joins("Relation")` | 写入关联表 ON |
| GORM 嵌套关联 | 展开每一级关联，分别写入对应 ON |
| 原生 INNER JOIN | 优先写入 ON；`USING` 可安全回退到 WHERE |
| 原生 LEFT JOIN + ON | 写入 ON，保留无关联数据的主表行 |
| 原生 RIGHT JOIN | 写入 WHERE，过滤作为保留表的受保护关联表 |
| CROSS JOIN | 写入 WHERE |
| 多个原生 JOIN | 逐段识别并分别追加隔离条件 |
| ON 中包含 OR | 原 ON 条件先整体加括号，再追加隔离条件 |
| 派生表或无法识别实际表名的原生 JOIN | 返回 `ErrRawDataIsolationUnsupported` |
| LEFT JOIN + USING、NATURAL LEFT JOIN、FULL OUTER JOIN | 无法安全保持外连接语义，返回 `ErrRawDataIsolationUnsupported` |

复杂 JOIN 被拒绝时，应改写为可识别的 GORM 关联或普通 JOIN；只有可信系统任务才允许显式跳过隔离。

## Row、Rows、Scan 和 Raw SQL

- 携带 token 时，`Find`、`First`、`Take`、`Count` 等普通 GORM 查询执行租户和数据范围隔离。
- 携带 token 时，`Rows`、`Scan` 使用独立 Row 回调，同样执行隔离；拒绝时返回对应错误。
- 携带 token 时，GORM `Row()` 没有 error 返回值，隔离拒绝时会执行恒不命中的查询，随后 `Scan` 返回 `sql.ErrNoRows`。
- 携带 token 时，`Raw(...)` 查询和 `Exec(...)` 无法可靠自动改写，默认返回 `ErrRawDataIsolationUnsupported`。
- 未携带 token 时，上述查询、写入与 Raw SQL 回调全部跳过隔离处理。

## 可信系统任务

携带认证声明的迁移、运维、跨租户统计等可信任务可显式跳过租户和数据范围隔离：

```go
var rows []ReportRow
err := db.Scopes(gormkit.SkipDataIsolation).
	Raw("SELECT tenant_id, COUNT(*) AS total FROM test GROUP BY tenant_id").
	Scan(&rows).Error
```

`SkipDataIsolation` 会同时跳过租户隔离、角色数据范围和 Raw SQL 拒绝逻辑，不应由普通请求参数控制。`NewGormClient` 内部的自动迁移和表注释回填已经自动使用该豁免。

## 错误

| 错误 | 含义 |
| --- | --- |
| `ErrTenantContextMissing` | 已携带 token，但受保护租户表缺少有效租户身份 |
| `ErrDataScopeContextMissing` | 数据范围回调无法从数据库语句中取得上下文 |
| `ErrRawDataIsolationUnsupported` | Raw SQL 或 JOIN 无法安全追加隔离条件，需要改写或由可信任务显式跳过 |

## 自动迁移和表注释

开启 `enable_migrate` 后，客户端会：

1. 获取当前客户端的 `WithMigrateModels` 模型；未设置时回退到 `RegisterMigrateModel` 或 `RegisterMigrateModels` 注册的模型。
2. 执行 GORM `AutoMigrate`。
3. 为实现 `TableCommenter` 的模型回填表注释。

表注释回填当前使用 `ALTER TABLE ... COMMENT = ...`，主要适用于 MySQL/Doris。其他数据库可以使用 `AutoMigrate`，但模型不应实现 `TableCommenter`，除非对应数据库确认支持该语法。

## 自定义回调

模块提供以下注册入口：

- `RegisterCallbackQuery`、`RegisterCallbackQueries`
- `RegisterCallbackRow`
- `RegisterCallbackRaw`
- `RegisterCallbackCreate`、`RegisterCallbackCreates`
- `RegisterCallbackUpdate`、`RegisterCallbackUpdates`
- `RegisterCallbackUpdateBefore`
- `RegisterCallbackDelete`、`RegisterCallbackDeletes`

自定义回调同样应在 `NewGormClient` 之前注册，已创建的客户端不会自动加载后续注册项。

## 索引建议

隔离条件会参与所有常规查询和写操作，业务表建议至少评估以下索引：

- 租户业务表：`(tenant_id, id)`。
- 按创建人查询频繁的表：`(tenant_id, created_by)`。
- `base_user`：主键 `id`，以及 `(tenant_id, dept_id, id)`。
- `base_dept`：主键 `id` 和租户索引；`path` 当前使用前置通配符 LIKE，普通 B-Tree 索引通常无法直接加速该条件。

最终索引应结合真实查询条件、数据量和数据库执行计划确定。
