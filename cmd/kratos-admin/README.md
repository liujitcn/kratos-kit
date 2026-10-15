# kratos-admin

`kratos-admin` 是仓库内的项目生成命令。它从内置模板创建一个包含后端、三端前端目录、配置、迁移、脚本和 Makefile 的新项目，并在生成阶段初始化前端和验证后端。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin@latest
```

## 使用

```bash
kratos-admin create shop-admin
kratos-admin create github.com/example/shop-admin --module github.com/example/shop-admin/backend
kratos-admin create shop-admin --modules system,order
```

参数：

| 参数 | 说明 |
| --- | --- |
| `create <project>` | 必填。项目名或包含路径的项目标识；目标目录已存在时拒绝覆盖。 |
| `--module` | 覆盖生成后端的 Go module 路径，默认使用项目路径加 `/backend`。 |
| `--modules` | 业务模块清单，逗号分隔；可重复指定，默认是 `system`。 |
| `--frontend-module` | `--modules` 的兼容名称。 |

业务模块名必须以小写字母开头，只能包含小写字母、数字和连字符，不能重复，也不能使用 `core`。

## 生成结构

```text
shop-admin
├── backend
│   ├── api/proto/<module>/{admin,app,config}/v1
│   ├── internal/{biz,data,service,server,task,module}
│   ├── configs
│   ├── migration/assets/v0.0.1/mysql
│   ├── scripts
│   └── Makefile
├── frontend
│   ├── admin
│   ├── uni-app
│   └── taro-app
├── scripts
├── Makefile
└── README.md
```

模板会为每个业务模块展开 Proto、Biz、Service、Server、Task 和前端模块目录；空目录通过 `.gitkeep` 保留。后端保持一个 Go module，服务入口、生成代码和业务代码共享同一 module。

## 生成阶段

命令按四个阶段执行：

1. 创建目录并渲染内置模板；
2. 使用三个前端 CLI 初始化 `frontend/admin`、`frontend/uni-app` 和 `frontend/taro-app`；
3. 解析并写入后端依赖版本，执行 `go mod tidy` 和代码生成；
4. 格式化并执行后端测试。

依赖版本查询有 20 秒超时。远端版本不可用时只使用完整的本地缓存；没有完整缓存会直接失败。任一阶段失败都会删除本次新建的未完成目录。命令会实时转发子命令输出，并每 10 秒打印一次仍在执行的状态。

## 生成后的常用命令

```bash
make -C backend init
make -C backend gen
make -C backend test
make -C frontend init
make -C frontend check
```

生成器不会覆盖已有目标目录，也不会把调用方的 Go workspace 传给后端命令；后端子命令使用 `GOWORK=off`。

## 验证生成器

```bash
GOWORK=off go test ./...
```

需要验证真实生成结果时，应在临时目录执行 `create`，检查后端 `go test ./...`、三个前端的检查命令以及生成项目的 Makefile 入口。
