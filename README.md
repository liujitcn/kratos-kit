# kratos-kit

`kratos-kit` 是一个基于 Kratos 的工具库集合，提供应用引导、配置加载、日志、注册发现、链路追踪，以及缓存/队列/鉴权/OSS/数据库等通用能力。

## 仓库说明

该仓库是多模块（multi-module）结构，根目录与子目录都包含 `go.mod`。常用模块包括：

- `api`：protobuf 定义与代码生成（`buf generate`）
- `bootstrap`：应用启动入口（配置加载 + 日志 + 注册中心 + tracer + `kratos.App`）
- `config`：本地/远程配置加载与工厂注册
- `logger`：日志工厂（`std`/`zap`/`logrus`/`fluent`/`aliyun`/`tencent`/`zerolog`）
- `registry`：注册发现工厂（`consul`/`etcd`/`eureka`/`kubernetes`/`nacos`/`polaris`/`servicecomb`/`zookeeper`）
- `tracer`：OpenTelemetry TracerProvider 与 exporter 工厂（`std`/`zipkin`/`otlp-http`/`otlp-grpc`）
- `tracing`：OpenTelemetry 追踪适配层
- `auth`：认证与鉴权封装（含 `authn`/`authz`、engine、middleware 子模块）
- `cache`：内存/Redis 缓存封装
- `queue`：内存/Redis 队列封装
- `locker`：Redis 分布式锁封装
- `oss`：本地/FTP/MinIO/阿里云 OSS 封装
- `database/gorm`：GORM 客户端封装（含多数据库 driver 子模块）
- `broker`：消息发布订阅与 typed handler 封装
- `transport`：通用传输辅助（含 `keepalive`、`mcp` 子模块）
- `rpc`：Kratos HTTP/GRPC Server 配置封装
- `swagger-ui`：Swagger UI 嵌入与路由注册封装（支持 `net/http` 与 Kratos）
- `pprof`：性能采样封装（当前支持 `pyroscope`）
- `captcha`：验证码生成与存储封装
- `sdk`：SDK 初始化入口封装
- `runtime`：运行时应用信息模型
- `utils`：通用工具（TLS、Redis 配置辅助）

## 安装

请按模块路径安装，而不是安装根模块。例如：

```bash
go get github.com/liujitcn/kratos-kit/bootstrap@latest
go get github.com/liujitcn/kratos-kit/config@latest
go get github.com/liujitcn/kratos-kit/logger@latest
go get github.com/liujitcn/kratos-kit/registry@latest
go get github.com/liujitcn/kratos-kit/tracer@latest
go get github.com/liujitcn/kratos-kit/transport/mcp@latest
go get github.com/liujitcn/kratos-kit/rpc@latest
go get github.com/liujitcn/kratos-kit/swagger-ui@latest
go get github.com/liujitcn/kratos-kit/pprof@latest
```

## 快速开始

### 1. 引入需要的实现包（通过 `init` 自动注册）

```go
import (
	_ "github.com/liujitcn/kratos-kit/config/etcd"
	_ "github.com/liujitcn/kratos-kit/logger/zap"
	_ "github.com/liujitcn/kratos-kit/registry/etcd"
)
```

### 2. 启动应用

```go
package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/liujitcn/kratos-kit/bootstrap"
)

func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	app := bootstrap.NewApp(ctx)
	return app, func() {}, nil
}

func main() {
	ctx := bootstrap.NewContext(nil, nil)
	if err := bootstrap.RunApp(ctx, initApp); err != nil {
		panic(err)
	}
}
```

默认命令行参数（`bootstrap/flag.go`）：

- `-c, --conf`：配置目录，默认 `../../configs`
- `-e, --env`：运行环境，默认 `dev`
- `-s, --chost`：配置中心地址，默认 `127.0.0.1:8500`
- `-t, --ctype`：配置中心类型，默认 `consul`
- `-d, --daemon`：以守护进程方式运行（非 Windows）

## 配置加载行为

`config.LoadBootstrapConfig(configPath)` 的行为：

1. 始终加载本地配置源（`configPath`）。
2. 若存在 `${configPath}/config.yaml`，先读取其中 `config.type`，再创建对应远程配置源并合并加载。
3. 扫描 `conf.Bootstrap` 及已注册的自定义配置结构。

远程配置源类型由 `config.type` 决定，可选值见 `config/types.go`：`apollo`/`consul`/`etcd`/`kubernetes`/`nacos`/`polaris`。

## API 代码生成

```bash
make api
```

`buf` 模块根目录为 `api/protos`，同模块 proto 引用使用模块内路径，例如：

```proto
import "conf/tls.proto";
```

## 开发命令

```bash
make plugin   # 安装 protoc 相关插件
make cli      # 安装 kratos/buf 等命令行工具
make fmt      # 使用 goimports 统一整理 Go 代码
make api      # 生成 api 代码
make gen      # 一键生成并整理 api 代码
make tag      # 默认从仓库根目录递归检查 go.mod 并自动打/推送 tag（含根模块）
make tag MODULE=auth       # 从 auth 目录开始递归检查 go.mod 并打 tag
make tag MODULE=auth/authn # 从 auth/authn 目录开始递归检查 go.mod 并打 tag
```

## 子模块文档

- [api/README.md](api/README.md)
- [bootstrap/README.md](bootstrap/README.md)
- [config/README.md](config/README.md)
- [logger/README.md](logger/README.md)
- [registry/README.md](registry/README.md)
- [tracer/README.md](tracer/README.md)
- [rpc/README.md](rpc/README.md)
- [swagger-ui/README.md](swagger-ui/README.md)
- [auth/authn/README.md](auth/authn/README.md)
- [auth/authz/README.md](auth/authz/README.md)
- [auth/authn/engine/jwt/README.md](auth/authn/engine/jwt/README.md)
- [auth/authz/engine/casbin/README.md](auth/authz/engine/casbin/README.md)
- [transport/keepalive/README.md](transport/keepalive/README.md)
- [transport/mcp/README.md](transport/mcp/README.md)

## 来源与版权说明

本仓库大部分代码参考或来源于以下开源项目，并在此基础上结合当前业务需求进行了整理与调整：

- [tx7do/kratos-bootstrap](https://github.com/tx7do/kratos-bootstrap)
- [tx7do/kratos-transport](https://github.com/tx7do/kratos-transport)
- [tx7do/kratos-authn](https://github.com/tx7do/kratos-authn)
- [tx7do/kratos-authz](https://github.com/tx7do/kratos-authz)

若涉及版权或授权边界问题，请优先以上游项目许可证与仓库声明为准，并联系维护者处理。
