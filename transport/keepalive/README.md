# transport/keepalive

`transport/keepalive` 把 gRPC health check 注册为 Kratos `transport.Server`，用于为注册发现或消息服务提供独立的存活端点。它只负责保活连接和生命周期，不承载业务 RPC。

## 创建

```go
srv := keepalive.NewServer(
	keepalive.WithNetwork("tcp"),
	keepalive.WithAddress(":9000"),
	keepalive.WithServiceKind("broker"),
)
app := kratos.New(kratos.Server(srv))
```

默认网络为 `tcp`。未设置地址时会生成本机地址和临时端口；通过 `WithEndpoint` 可以直接指定注册发现使用的完整 endpoint，通过 `WithTLSConfig` 启用 TLS。

`Start` 创建监听器、注册 gRPC health server 并阻塞服务；`Stop` 暂停健康检查并优雅停止 gRPC server。`Endpoint` 返回可用于注册发现的 `keepalive://...` URL，并实现 `transport.Endpointer`。

## 地址选择

当监听地址只包含端口时，模块会根据实际 listener 调整 endpoint。多网卡或容器环境无法自动选择可达地址时，可以设置：

```bash
export KRATOS_TRANSPORT_KEEPALIVE_INTERFACE=eth0
export KRATOS_TRANSPORT_KEEPALIVE_HOST=192.0.2.10
```

设置接口名时优先使用接口地址；否则使用显式 Host，最后才自动探测本机地址。环境变量只影响 endpoint 地址推导，不会改变 gRPC 监听端口。调用方仍应确认该地址能被注册中心或探活方访问。
