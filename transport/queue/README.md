# kratos-kit queue transport

`transport/queue` 将现有 `github.com/liujitcn/kratos-kit/queue` 适配为 Kratos `transport.Server`，不修改原 queue 模块。

## 创建

默认使用本地内存队列：

```go
queueSrv, err := queueTransport.NewServer(queueTransport.WithMemory(128))
```

使用 Redis：

```go
queueSrv, err := queueTransport.NewServer(queueTransport.WithRedis(redisConfig, queueConfig))
```

注册处理器后，将 `queueSrv` 传给 `kratos.Server(queueSrv)`，应用会统一调用 `Start` 和 `Stop`。也可以使用 `WithQueue` 注入一个已有的 `queue.Queue` 实例。

```go
const orderStream queueTransport.Stream = "orders"

queueSrv.Register(orderStream, func(message queueData.Message) error {
	return handleOrder(message)
})
```

示例中的导入别名：

```go
queueTransport "github.com/liujitcn/kratos-kit/transport/queue"
queueData "github.com/liujitcn/kratos-kit/queue/data"
```

安装：

```bash
go get github.com/liujitcn/kratos-kit/transport/queue@latest
```
