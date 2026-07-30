# NATS broker

该模块通过同一个 adapter 支持 Core NATS 与 JetStream，直接实现
`github.com/liujitcn/kratos-kit/broker.Broker`。

```go
instance := nats.NewBroker(
	broker.WithAddress("nats://127.0.0.1:4222"),
	broker.WithCodec("json"),
)
if err := instance.Connect(); err != nil {
	return err
}
defer instance.Disconnect()
```

持久消息使用 `nats.NewJetStreamBroker`。请求超时直接使用公共的
`broker.WithRequestTimeout`，不再维护 NATS 专用的重复选项。配置
`broker.WithReplyTopic` 时，NATS adapter 将该值作为前缀，为每次请求派生唯一 reply
subject，避免并发请求接收同一响应。
