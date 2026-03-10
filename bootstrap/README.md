# 应用程序引导


## 概述

此包负责程序的引导配置管理。提供一个线程安全的初始化流程和配置注册机制，用于在应用启动阶段集中管理各类配置结构体（例如服务器、客户端、数据、日志等）。

## 设计要点

- 延迟初始化：使用 `sync.Once` 确保引导配置仅初始化一次。
- 并发安全：读写操作通过 `sync.RWMutex` 保护。
- 配置注册：通过 `RegisterConfig` 注册任意非空指针类型配置（例如 `&conf.SomeConfig{}`），内部对同一指针地址做去重。
- 主配置访问：使用 `GetBootstrapConfig` 获取共享的 `*conf.Bootstrap` 实例。

## 使用示例

```go
package main

import (
    "github.com/liujitcn/kratos-kit/bootstrap"
    "github.com/liujitcn/kratos-kit/bootstrap/api/gen/go/conf"

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
	lg log.Logger,
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
