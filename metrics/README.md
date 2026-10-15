# metrics

`metrics` 定义与后端无关的 `Metrics` 接口：`Counter` 记录单调递增值，`Histogram` 记录分布，`Gauge` 设置瞬时值。标签使用 `map[string]string` 传入。

## Provider

| 模块 | 后端 | 关闭行为 |
| --- | --- | --- |
| `metrics/prometheus` | 独立 Registry 或进程默认 Registry | 无后台连接，通常不需要关闭。 |
| `metrics/otel` | OTLP gRPC 或 HTTP | `Close` 刷新并关闭 MeterProvider。 |
| `metrics/datadog` | DogStatsD | `Close` 刷新并关闭客户端。 |

```go
provider, err := prometheus.New(
	prometheus.WithNamespace("order"),
	prometheus.WithSubsystem("http"),
)
if err != nil {
	return err
}
provider.Counter(ctx, "requests_total", 1, map[string]string{"method": "GET"})
```

Prometheus Provider 会按首次使用时的标签名建立指标，后续同名指标应保持相同标签集合。OTLP 和 DogStatsD Provider 应在应用退出时调用 `Close`。
