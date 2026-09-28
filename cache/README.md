# Cache

本模块提供内存、Redis 和 Redis Cluster 缓存实现。

## 限流

`RateLimitStore.TakeRateLimits` 支持令牌桶、固定窗口、滑动窗口计数、滑动窗口日志和漏桶。一次请求可组合多个键，只有全部通过时才会原子更新状态；被拒绝时返回最长重试等待时间。

`Cache.TakeTokenBuckets` 保留便捷的令牌桶接口，并复用统一限流实现。Redis 使用 Lua 脚本执行原子检查与更新；Redis Cluster 的同一批键必须带有相同的 Redis hash tag。

## 共享缓存版本

`ReadRevision` 读取数值版本，键不存在时按 `0` 处理；版本无效或缓存读取失败时返回不可用。`IncrementRevision` 原子递增版本，使其他节点切换到新缓存键。
