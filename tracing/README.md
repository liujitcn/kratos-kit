# tracing

`tracing` 是 Kratos transport 的 OpenTelemetry 适配层，提供服务端、客户端、生产者和消费者 span，以及上下文传播。

## Middleware

```go
serverMiddleware := tracing.Server()
clientMiddleware := tracing.Client()
```

服务端和客户端 middleware 会从 Kratos transport 读取 operation、kind 和 endpoint，生成对应 span；服务端/消费者从请求载体提取 TraceContext，客户端/生产者把当前上下文注入载体。处理器返回错误时 span 会标记为 error。

## 手动使用

`NewTracer` 创建指定 `SpanKind` 的 tracer，`Start` 或 `StartWithName` 开始 span，`End` 设置错误状态并结束 span。`NewTracerProvider` 可按 exporter、endpoint、服务名、实例 ID、版本和采样率创建 SDK Provider。

本模块只负责传输适配和 span 生命周期；exporter 的具体配置可使用 [`tracer`](../tracer/README.md)。
