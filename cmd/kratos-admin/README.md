# kratos-admin

`kratos-admin` 用于创建包含前后端的完整项目。项目根目录和后端由本命令内置模板
生成；前端全部由三个 npm CLI 生成。Go 只解析版本、传入模块与集成参数并执行 CLI，
不内置前端模板，不重命名或改写前端文件。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin@latest
```

## 使用

传入项目名即可创建携带 Admin 登录、用户、角色、菜单、权限、文件、日志、任务和消息能力的完整项目：

```bash
kratos-admin create shop-admin
```

命令会创建以下结构：

```text
shop-admin
├── backend
│   ├── api/proto/<module>/{admin,app,config}/v1
│   ├── internal/{biz,data,service,server,task,module}
│   ├── internal/i18n/assets
│   ├── internal/openapi/assets
│   ├── migration/assets/v0.0.1/mysql
│   ├── configs
│   ├── scripts
│   └── Makefile
├── frontend
│   ├── admin     # @liujitcn/kratos-admin-cli
│   ├── uni-app   # @liujitcn/kratos-uni-app-cli
│   └── taro-app  # @liujitcn/kratos-taro-app-cli
├── scripts       # Git hooks、国际化、证书和发布脚本
├── frontend/Makefile
├── frontend/scripts
├── Makefile
└── README.md
```

业务 module 默认是 `system`，与项目名独立，同时用于前端与后端业务目录。
也可以直接传入仓库路径：

```bash
kratos-admin create github.com/example/test
```

此时项目目录为 `test`，业务 module 为 `system`，后端 `backend/go.mod` 的 module 为
`github.com/example/test/backend`。仅传项目名时，Go module 默认为
`github.com/example/<project>/backend`。仍可显式覆盖后端 Go module 和业务 module：

```bash
kratos-admin create shop-admin \
  --module github.com/acme/shop-admin/backend \
  --modules system,order
```

多个业务模块使用逗号分隔，也可以重复传参：

```bash
kratos-admin create github.com/example/test --modules system,order
kratos-admin create github.com/example/test --modules system --modules order
```

兼容 `--frontend-module` 参数，`--module` 仍仅用于覆盖后端 Go module 路径。
显式指定模块清单时不会额外添加 `system`。三端各保留一个宿主，每端都生成
`packages/modules/system`、`packages/modules/order` 等业务包并注册到模块清单。
后端逐模块生成 biz/service/server、Proto 目录和三端 Buf 配置；Wire、国际化、打包与发布覆盖全部模块。
本地管理端 `system` 继承内置 System 能力并合并本地视图，运行时只注册一次。
内置 System 包仍参与 Vite 源码扫描、自动导入和 Swagger 依赖预构建。
三端所有本地业务模块都会注册脚本生成的语言资源；管理端 `system` 按语言合并内置与本地文案，
避免启动时出现 `User is not defined` 或 `@local/system 缺少 zh-CN 语言包`。

生成时不预创建后端根目录下的 `adapter`、`client`、`backups`、`codegen`、`data`、
`logs`；配置和构建脚本保留运行时路径，实际使用时再创建对应目录。

后端 `biz`、`service`、`server` 的各级业务目录，以及 `config`、`task`、`data`、
`module` 默认提供 `init.go` 和 Wire `ProviderSet`，逐层汇总到 `internal/module`。
Proto、配置资源、脚本和生成代码目录不放置手写 Go 初始化文件。

生成过程会依次调用以下 CLI，并迁移后端、前端各自的 Makefile 与脚本入口：

- `@liujitcn/kratos-admin-cli`
- `@liujitcn/kratos-uni-app-cli`
- `@liujitcn/kratos-taro-app-cli`

三端 CLI 直接接收 `--module system` 等业务模块参数，并启用 `--kratos-project`：
该模式统一 H5 输出目录，管理端 CLI 还生成共享 `frontend/Makefile` 和 `frontend/scripts`。
CLI 原生支持本地 `system`，不再使用临时模块名。需先发布支持新参数的三端 CLI，
再通过 Go 命令从 npm 创建项目；本地回归测试可以验证尚未发布的 CLI 源码。

每次生成先从 npm 官方源查询各 CLI 的 `dist-tags.latest`，再通过 `pnpm dlx 包名@精确版本`
执行。取消强制刷新 dlx 缓存，同版本按 pnpm 的缓存有效期复用，新版本使用独立缓存。
`@liujitcn` 作用域仅通过命令行参数临时指向 npm 官方源，其他作用域和全局配置保持原样。

后端通过 `git ls-remote` 查询 `backend/v*` tag，按语义版本选择当前模块可用的最新稳定版本，
与 `go env GOMODCACHE` 下的源码和下载缓存比较。已缓存时通过 `go mod edit -require=...@版本`
写入依赖并跳过 `go get`，未缓存时执行 `go get ...@精确版本`。不再执行 `go get ...@latest`。
Git 与 npm 元数据查询均限时 20 秒。Git 查询失败时改用 Go 代理查询（同样限时 20 秒）；
两者都不可用时，明确提示无法确认最新版本，并使用本地完整缓存中的最高稳定版本继续生成。
仅有不完整缓存或完全无缓存时仍报错；npm 查询失败仍明确报错。
仍使用 Backend 自身声明的 Admin API 版本，避免强制覆盖依赖导致跨版本组合。
随后执行后端 `go mod tidy`，通过固定版本 Wire 生成内部模块与服务入口，再格式化和执行 `go test ./...`。
`tidy` 仍可能下载本地缺失的间接依赖；缓存命中不代表整个生成流程完全离线。
前端 CLI 自行生成工具链和语言注册文件。任一前端 CLI 或初始化步骤失败，
本命令都会清理本次新建的不完整项目目录。

终端会显示模板、前端 CLI 生成、依赖解析、Wire 与后端验证四个阶段。
每条子命令显示工作目录，实时输出标准输出和标准错误；执行期间每 10 秒报告当前命令和
耗时，结束后显示完成或失败状态，避免下载依赖或编译期间长时间没有反馈。

### 后端模块边界

后端只有 `backend/go.mod` 一个模块，保留用户指定的 module 名；服务入口
`backend/internal/cmd/server` 属于同一模块，不生成额外的宿主 `go.mod`、`go.work` 或本地替换。

Admin 的公开 `adapter/core` 和 `adapter/kit` 构造函数统一接收数据库客户端，在函数内部初始化
所需仓储。Wire 只使用公开适配器和 Core/Kit 接口，生成项目不会引用 Admin 的 `internal` 包，
也不复制依赖源码。脱敏策略通过实例和请求上下文传递，不依赖全局默认解析器。

在 Backend 目录执行 `go test ./...` 或 `make test` 会同时覆盖业务代码和服务入口，构建使用
`make build`。初始化时设置 `GOWORK=off`，避免继承调用方的 Go workspace。
后端 Makefile 同样固定 `GOWORK=off`；`make cli` 安装 golangci-lint v2，以兼容 Go 1.27。

生成项目沿用 Admin 的 MySQL、Redis、Consul 和 Vault 配置结构，不复制私有环境配置或凭据。
先准备基础设施和 Vault 访问凭据，再执行 `make -C backend init`、`make -C frontend init`。
新目录尚未初始化 Git 时先执行 `git init`，根目录 `make init` 只安装 Git hooks，与 Admin 一致。

### 模板同步基准

当前基准是 `kratos-admin v0.0.37`（`70be66e346dc5c6e811a7fa8e45d140f91773ab4`）。
三层 Makefile 的目标名称、所在层级和执行顺序对齐该版本，不保留旧的根目录 `run/infra-up`
或后端 `ts/docker-build` 转发目标。

| 范围 | 同步方式 |
| --- | --- |
| Dockerfile、dockerignore、入口脚本、证书脚本、OpenAPI 多语言工具、重装脚本、Git hook | 复用基准公共文件，保留可执行权限 |
| 根/后端 Makefile、发布脚本 | 替换项目名称、业务包清单、Proto 输入与输出目录；只发布自己的业务包 |
| 前端 Makefile、发布与重装脚本 | 由管理端 npm CLI 的 `--kratos-project` 模式生成 |
| 基础 configs、Go/OpenAPI Buf 配置 | 同步通用字段，保留项目数据库参数与空 AI 配置；不复制 `*.dev.yaml` |
| 三端 RPC 配置 | 从对应上游模板派生，只生成当前业务模块，不生成 npm Core/System 包的源码 |
| 语言工具 | 保留校验与生成逻辑，目录改为项目业务模块；空业务不要求 Admin 专属翻译 SQL/代码生成文案 |
| 前端宿主与检查 | 三端 npm CLI 直接生成生命周期、语言注册、自动导入、tsconfig、lint、测试和打包配置 |

`Resource` 的 `Models/I18n/OpenAPI/Migrations` 均已挂接。模型和语言初始为空，migration 仅保留
说明文件，目录通过占位文件保留。添加表后在 `internal/data.Models()` 汇总模型；项目迁移依赖 Admin。
没有业务表或 Proto 时，对应生成目标明确跳过。安装工具时补齐这些命令实际依赖的 Wire 和脱敏插件。

三个 H5 宿主统一输出到 `backend/data/{admin,uni-app,taro-app}`，由根 Makefile 构建并进入 Docker
静态资源种子目录。管理端宿主不缓存 workspace 之外的 H5 输出，避免命中缓存后缺少 Docker 构建资源。

## 验证生成器

```bash
GOWORK=off go test ./...
GOWORK=off KRATOS_ADMIN_SOURCE_DIR=/path/to/kratos-admin go test -run TestMakefileTargetsMatchAdmin -v
GOWORK=off KRATOS_ADMIN_SOURCE_DIR=/path/to/kratos-admin go test -run TestFrontendRuntimeRegistration -v
GOWORK=off KRATOS_ADMIN_INTEGRATION=1 go test -run TestGeneratedBackendBuilds -v
```

集成测试需要 Go、Make 和 Wire，跳过前端 CLI，真实生成默认及自定义 `/v2` module 的后端，
执行 Wire、`go test ./...` 和生成项目的 `make test build`，并检查 Wire 产物不引用 Admin 内部包。
依赖解析沿用当前 Go 代理设置。测试未发布的跨仓库修复时，可额外设置
`KRATOS_ADMIN_BACKEND_DIR`、`KRATOS_CORE_DIR`、`KRATOS_KIT_REDACT_DIR` 和 `KRATOS_KIT_GRPC_DIR` 指向对应本地模块；
这些替换仅作用于测试的临时项目。

前端运行时回归测试使用指定源码仓库的三端 CLI（管理端 CLI 需先构建，并安装管理端工具依赖），
真实生成 `system,order` 模块，检查语言注册、内置文案保留、System 单次注册和自动导入转换。
该测试不连接后端，不替代实际环境的登录联调。

完整验证还应实际创建项目，执行三端 `make -C frontend init/check/build-h5/build-mp-weixin`、
`make -C backend gen/test/package-binary`、`make i18n I18N_OFFLINE=1` 和 `make i18n-verify`，
并用临时业务 Proto 验证 Go、脱敏、OpenAPI 和 TypeScript RPC 生成。发布及容器启动不属于验证步骤。
