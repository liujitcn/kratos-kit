# retry

`retry` 提供 context 感知的通用重试器。调用方提供操作函数和错误分类器，模块按最大次数、总等待时间、退避策略和抖动策略决定是否再次执行。

## 退避策略

- `FixedBackoff`：固定等待时间；
- `LinearBackoff`：按步长线性增加；
- `ExponentialBackoff`：按倍数指数增加并可设置上限；
- `NoJitter`、`FullJitter`、`EqualJitter`：控制等待时间随机化。

```go
retrier := retry.New(
	retry.WithMaxAttempts(3),
	retry.WithBackoff(retry.ExponentialBackoff{Initial: 100 * time.Millisecond}),
)
err := retrier.Do(ctx, func(ctx context.Context) error {
	return callRemote(ctx)
})
```

调用方取消 context 时返回 context 错误；耗尽次数或总等待时间后分别返回 `ErrMaxAttempts` 或 `ErrTimeout`，并保留最后一次操作错误。
