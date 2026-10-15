# broker

`broker` 定义消息发布订阅的通用接口，并把消息后端接入 Kratos 应用生命周期。具体协议实现位于子模块，例如 [broker/nats](nats/README.md)。

## 核心接口

`Broker` 统一提供 `Init`、`Connect`、`Disconnect`、`Publish`、`Subscribe` 和 `Request`。后端不支持请求响应时返回可通过 `errors.Is(err, broker.ErrRequestUnsupported)` 判断的错误。

`Message` 携带消息体和 Header，`Event` 携带主题和消息，`Binder` 为订阅处理器创建消息体目标。`Subscribe[T]` 会把 `T` 或 `*T` 消息体转换为 typed handler 所需的 `*T`。

## 生命周期适配

```go
subscription := broker.NewSubscription(
	"orders.created",
	func(ctx context.Context, event broker.Event) error { return nil },
	func() any { return new(OrderCreated) },
)
server := broker.NewTransportServer(instance, subscription)
app := kratos.New(kratos.Server(server))
```

`TransportServer.Start` 会初始化、连接并建立预注册订阅；任一订阅失败会断开连接并返回错误。`Stop` 断开连接，同一实例可以再次启动。

## 中间件与扩展

发布和订阅都支持选项链。发布 middleware 可以包装 `PublishHandler`，订阅选项可控制消费组、并发、重试和消息确认行为，具体语义由后端实现决定。
