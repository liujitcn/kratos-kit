# 应用程序引导


## 概述

此包负责程序的引导配置管理。提供一个线程安全的初始化流程和配置注册机制，用于在应用启动阶段集中管理各类配置结构体（例如服务器、客户端、数据、日志等）。

## 设计要点

- 延迟初始化：使用 `sync.Once` 确保引导配置仅初始化一次。
- 并发安全：读写操作通过 `sync.RWMutex` 保护。
- 配置注册：通过 `RegisterConfig` 注册任意非空指针类型配置（例如 `&configv1.SomeConfig{}`），内部对同一指针地址做去重。
- 主配置访问：使用 `GetBootstrapConfig` 获取共享的 `*configv1.Bootstrap` 实例。

## app-info 初始化

启动时按以下优先级初始化 app-info 的五个基础字段：启动参数 → `NewContext` 传入的 `*configv1.AppInfo` → 默认常量。

可分别传入以下启动参数：

```bash
go run . \
  -p shop \
  -a admin-service \
  -i shop-admin-service@host \
  -n "Shop Admin Service" \
  -v v1.2.3
```

对应长参数为 `--project`、`--app-id`、`--instance-id`、`--name` 和 `--version`，短参数分别为 `-p`、`-a`、`-i`、`-n` 和 `-v`；未传入的字段继续从 `AppInfo` 或默认值补齐。

## 运行环境配置

`--env`（短参数 `-e`）用于选择配置目录中的环境覆盖文件，默认值为 `dev`。基础文件始终加载，`<name>.<env>.yaml` 在基础文件之后加载并覆盖同名字段，其他环境文件会被忽略。

```text
configs/
├── data.yaml
├── data.dev.yaml
└── data.prod.yaml
```

```bash
go run . --conf configs --env dev
go run . -c configs -e prod
```

如果 `data.prod.yaml` 不存在，`env=prod` 会直接使用 `data.yaml`。环境覆盖文件可以只配置与基础文件不同的字段。

## 密钥 Provider

独立的 `key.yaml` 用于描述密钥 Provider。密钥配置只应包含类型、范围、根密钥引用
和 Provider 的非敏感连接参数；根密钥与 Provider 认证信息必须由外部 Secret Manager 或工作负载身份提供。

```yaml
type: vault
scope: prod/order-service
root_name: secret/data/kratos/prod/root
root_version: "3"
vault:
  address: http://127.0.0.1:8200
  value_key: value
```

启动时 bootstrap 先读取 `key.yaml`；如果 `sdk.Runtime` 已设置 Key 实例就直接复用，否则按 key 配置创建 Key，
没有 key 配置时默认使用 `configs/root.key` 的 file provider，随后使用派生的 config 密钥加载业务配置。
因此未配置 `key.yaml` 时，需要先将 32 字节根密钥保存为 `configs/root.key`；系统不会自动生成或覆盖根密钥。

## 使用示例

```go
package main

import (
    "log/slog"

    "github.com/liujitcn/kratos-kit/bootstrap"
    configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"

	//_ "github.com/liujitcn/kratos-kit/config/apollo"
	//_ "github.com/liujitcn/kratos-kit/config/consul"
	_ "github.com/liujitcn/kratos-kit/config/etcd"
	//_ "github.com/liujitcn/kratos-kit/config/kubernetes"
	//_ "github.com/liujitcn/kratos-kit/config/nacos"
	//_ "github.com/liujitcn/kratos-kit/config/polaris"

	//_ "github.com/liujitcn/kratos-kit/logger/aliyun"
	//_ "github.com/liujitcn/kratos-kit/logger/fluent"
	//_ "github.com/liujitcn/kratos-kit/logger/logrus"
	//_ "github.com/liujitcn/kratos-kit/logger/tencent"
	//_ "github.com/liujitcn/kratos-kit/logger/zap"
	//_ "github.com/liujitcn/kratos-kit/logger/zerolog"
	
	//_ "github.com/liujitcn/kratos-kit/registry/consul"
	_ "github.com/liujitcn/kratos-kit/registry/etcd"
	//_ "github.com/liujitcn/kratos-kit/registry/eureka"
	//_ "github.com/liujitcn/kratos-kit/registry/kubernetes"
	//_ "github.com/liujitcn/kratos-kit/registry/nacos"
	//_ "github.com/liujitcn/kratos-kit/registry/polaris"
	//_ "github.com/liujitcn/kratos-kit/registry/servicecomb"
	//_ "github.com/liujitcn/kratos-kit/registry/zookeeper"
)

var version string

// go build -ldflags "-X main.version=x.y.z"

func newApp(
	lg *slog.Logger,
	re registry.Registrar,
	hs *http.Server,
) *kratos.App {
	return bootstrap.NewApp(
		lg,
		re,
		hs,
	)
}

func main() {
	bootstrap.Bootstrap(initApp, trans.Ptr(service.AdminService), trans.Ptr(version))
}
```
