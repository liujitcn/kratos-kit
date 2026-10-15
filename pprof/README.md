# pprof

`pprof` 封装 Pyroscope 性能采样器的创建和生命周期。当前支持 `pyroscope` 类型，配置由 `config.v1.Pprof` 提供。

```go
profiler, err := pprof.NewPprof(cfg)
if err != nil {
	return err
}
if profiler != nil {
	profiler.Start()
	defer profiler.Stop()
}
```

Pyroscope 配置支持应用名、服务地址、Basic Auth、租户、上传频率、标签、采样类型、GC 行为和自定义 HTTP Header。启动或停止失败会记录日志，不会在调用方 goroutine 中 panic。
