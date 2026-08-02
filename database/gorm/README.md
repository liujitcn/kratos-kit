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
| 版本化迁移 | 已注册的 `migration/assets` SQL 默认执行，不受 `enable_migrate` 影响 |
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
`embed.FS` 提供按版本目录组织的迁移资源，通过 `Contributor.Name()`（类型为
`migration.ModuleName`）标识模块，
并可通过 `Dependencies` 声明执行顺序。
kit 只定义 `migration.ModuleName` 类型，不内置具体业务模块枚举。每个调用方应在自己的
模块包中声明 `migration.ModuleName` 常量，禁止在调用处传裸字符串；同一个 `Registry`
会拒绝重复注册的模块名，并校验模块名及其依赖的格式。
所有模块统一使用默认数据源中的 `base_migration` 表记录版本。单库配置使用
`data.database`，命名多数据源配置使用 `data.databases.default`。记录使用
`module` 区分迁移模块，使用 `data_source` 区分目标数据源，并保存 `description` 描述。
资源目录最外层只放版本目录，支持兼容的纯数字格式（如 `000001`）以及
`v0.0.1`、`v0.0.1-20260511170946`、`v0.0.1.20260511170946` 三种版本格式。
每个版本目录下按真实数据库类型建立 `mysql` 或 `doris` 目录。数据库类型目录下的直系文件
属于 `default` 数据源，一级子目录名表示数据源名称，例如：

```text
assets/v0.0.1/
  mysql/
    default-data.description.md
    default-data.up.sql
    shop/
      shop.description.md
      shop.up.sql
  doris/
    default-data.description.md
    default-data.up.sql
```

迁移执行器读取客户端配置声明的真实驱动，而不是只读取 GORM Dialector 名称，因此 Doris
复用 MySQL Dialector 时仍只会执行 `doris` 目录中的脚本。目标客户端必须使用 `mysql` 或
`doris` 驱动；目录名匹配已注入客户端时使用对应数据源，找不到匹配客户端时回退到 `default`，
若默认客户端的驱动与脚本类型不匹配仍会直接返回错误。

每个数据源资源都会产生独立的 `module + data_source + version` 迁移记录。数据库类型目录中的
`*.up.sql`、`*.down.sql` 和 `.md` 文件均为可选；所有 `.md` 文件按文件名排序后直接
拼接为对应数据源的版本描述。`*.down.sql` 文件只保存，不参与当前升级执行。多个升级
脚本按文件名排序，在同一个数据库事务中执行，任一脚本失败时回滚该版本的脚本并直接返回错误，
阻止应用启动。目录可以只有描述、只有脚本或完全为空；版本记录仅在该版本脚本全部执行成功后写入。

`base_migration` 对应的 `BaseMigration` 模型位于 `database/gorm/migration`。宿主项目
应在默认客户端的 GORM 模型中注册该模型；默认客户端在 `enable_migrate: true` 时通过
`AutoMigrate` 创建或更新表。版本化 SQL 迁移不读取 `enable_migrate`，默认中心数据库
默认中心数据库客户端必须在执行脚本前注入；目录声明的数据源客户端未注入时回退到默认客户端。默认库必须在执行脚本前已有
`base_migration` 表。MySQL 和 Doris 客户端默认启用多语句执行，以支持一个迁移文件包含
多条 SQL。
`BaseMigration.DataSource` 映射数据库中的 `data_source` 列；迁移记录不保存成功失败状态，
只有脚本全部成功后才会写入版本记录。
`base_migration.up_sql` 保存本次版本实际执行的全部升级脚本，`down_sql` 预留给后续回退能力，
当前不会执行回退脚本。升级脚本执行失败不会写入版本记录，应用启动失败；修复脚本后重新启动
会再次执行该版本。

```go
type contributor struct{}

const moduleName migration.ModuleName = "test"

// Name 返回迁移模块名称。
func (contributor) Name() migration.ModuleName {
	return moduleName
}

// Migrations 返回业务模块的版本化迁移资源。
func (contributor) Migrations() []migration.Migration {
	return []migration.Migration{{
		FS:   migrationFS,
		Path: "assets",
	}}
}

registry, err := migration.NewRegistry(
    migration.AdditionalMigrations{contributor{}},
)
runner, err := migration.NewRunner(registry)
// defaultClient 和 shopClient 为宿主创建的命名 *gorm.Client。
err = runner.SetClient(defaultClient)
err = runner.SetClient(shopClient)
err = runner.Run(ctx, moduleName)
```

迁移核心按版本目录中的数据源名称查找已注入客户端，未找到时使用 `default` 客户端。
`Run(ctx, moduleName)` 执行模块的全部数据源资源；传入一个客户端时可只执行该数据源，例如
`Run(ctx, moduleName, shopClient)`。依赖模块继承当前数据源，依赖模块如果也需要执行到
`shop`，应在对应版本目录下提供 `shop` 子目录。不要求接入项目依赖 kratos-admin 或 Wire。

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
