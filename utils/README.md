# utils

`utils` 提供各模块共用的轻量辅助函数：

- `LoadServerTlsConfig`、`LoadClientTlsConfig`：从文件或内存 PEM 构造 TLS 配置；
- `GetRedisOptions`、`GetClusterRedisOptions`、`GetUniversalOptions`：从 `config.v1.Data_Redis` 创建 go-redis 连接选项；
- `ToolNameFromRPCPath`：把 `/package.Service/Method` 转为稳定的 snake_case Tool 名称。

```go
name := utils.ToolNameFromRPCPath("/orders.v1.OrderService/GetOrder")
// orders_v1_order_service_get_order
```

Redis 地址为空或单机配置缺少第一个地址时会返回错误。TLS 只在配置存在时创建，调用方可将返回的 `*tls.Config` 交给 HTTP、gRPC 或 Redis 客户端。
