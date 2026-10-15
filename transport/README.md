# transport

`transport` 提供可挂入 Kratos 生命周期的通用传输扩展。公共包负责注册和 transport 工具，具体能力按子模块拆分：

| 子模块 | 能力 |
| --- | --- |
| `transport/cron` | 秒级 Cron、动态任务、时区和优雅停止。 |
| `transport/hptimer` | 单次、间隔和 Cron 高精度定时任务。 |
| `transport/keepalive` | 基于 gRPC health check 的保活服务。 |
| `transport/mcp` | MCP Streamable HTTP、Legacy SSE、进程内和 STDIO。 |
| `transport/queue` | 把 `queue.Queue` 适配为 Kratos `transport.Server`。 |
| `transport/sse` | SSE 服务端、客户端、事件流、认证和断线处理。 |

详细配置和生命周期语义见各子模块 README。所有实现都应通过 `Start`/`Stop` 接入同一个 Kratos App，避免重复创建监听端口或后台 goroutine。
