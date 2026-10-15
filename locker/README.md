# locker

`locker` 定义统一的锁接口，并提供进程内和 Redis 分布式实现。

## 创建

```go
lock, err := locker.NewLocker(redisConfig)
```

传入 Redis 配置时创建 Redis 锁；配置为空时使用进程内锁。也可以直接使用 `locker/memory.New()` 或 `locker/redis.New()` 获得具体实现。

## 使用

锁接口支持按 key 获取和释放锁，Redis 实现使用租约时长避免永久占用。业务应保证释放锁与获取锁使用同一 key，并为跨进程临界区设置明确的超时时间。

本模块只负责互斥，不负责业务幂等、事务提交或死信处理。
