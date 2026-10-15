# queue

`queue` 提供统一的消息队列接口，支持内存队列和 Redis Stream 队列；Redis 实现还支持延迟消息。

## 创建

```go
queue, cleanup, err := queue.NewQueue(redisConfig, queueConfig)
if err != nil {
	return err
}
defer cleanup()
```

Redis 配置为空时使用内存队列；配置存在时使用 Redis Stream。通过 `Append` 写入消息，通过 `Register` 注册 stream 消费函数，再调用 `Run` 开始消费，关闭时调用 `Shutdown`。

`DelayedQueue` 提供 `Schedule` 和 `Cancel`。Redis 延迟消息由独立调度循环投递回目标 stream；生产者默认使用 `MAXLEN` 控制 stream 长度，消费者成功处理后确认并删除消息实体。

消息载荷是 `queue/data.Message`，包含 `ID`、`Values` 和 `ErrorCount`。调用方应让消费处理具备幂等性，并在进程退出时执行 `cleanup` 等待已拉取消息完成。
