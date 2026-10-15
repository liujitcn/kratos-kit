# health

`health` 聚合多个命名检查器，适合暴露应用的 liveness 和 readiness 探针。

## 检查器

实现 `Checker.Check(context.Context) Result` 即可注册。`PingFunc` 可把返回 `error` 的函数转换为检查器。`Register` 会新增或替换同名检查器，`Deregister` 删除检查器，`Names` 按名称排序返回当前清单。

`Health.Check` 并发执行所有检查器，默认总超时为 5 秒，可通过 `WithTimeout` 调整。任一检查为 `down` 时总体为 `down`；没有检查器时返回 `up`。

## HTTP handler

```go
h := health.New()
h.Register("database", health.PingFunc(func(ctx context.Context) error {
	return db.PingContext(ctx)
}))

http.Handle("/livez", health.NewLivenessHandler())
http.Handle("/readyz", health.NewHandler(h))
```

Liveness 始终返回 `200`。Readiness 在 `up` 或 `unknown` 时返回 `200`，在 `down` 或检查超时时返回 `503`，响应体为 JSON。
