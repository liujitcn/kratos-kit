# ratelimit

`ratelimit` 定义供 Kratos 服务端 middleware 组合使用的限流接口，并提供 Sentinel 和令牌桶两种实现。

## 实现

- `ratelimit/sentinel`：连接 Sentinel 客户端，按资源名执行流控。
- `ratelimit/tokenbucket`：进程内令牌桶，支持速率、突发容量、等待或立即拒绝。

调用方通常通过 middleware 包装请求处理器，达到限流条件时返回统一错误。令牌桶状态保存在进程内，多个实例需要各自配置容量；需要共享状态时可结合 `cache` 提供的限流存储能力。

具体选项以对应子模块的导出类型为准，服务端配置应把限流资源名与业务路由保持稳定映射。
