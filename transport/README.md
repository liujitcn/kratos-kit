# transport

`transport` 提供可挂入 Kratos 生命周期的通用传输扩展。公共包负责注册和 transport 工具，并提供两个可直接导入的安全工具包：

- `transport/clientip`：默认返回 TCP 对端地址，只有对端命中 `SECURITY_TRUSTED_PROXY_CIDRS` 时才从右向左解析 `X-Forwarded-For`，缺少该头时使用 `X-Real-IP`。
- `transport/safehttp`：在请求和拨号阶段校验目标，固定连接到已验证 IP，并校验每次重定向；默认拒绝私网、回环、链路本地和元数据地址。私网及回环服务需显式授权 CIDR，链路本地和元数据地址始终拒绝。

`safehttp.FromEnvironment(scope, allowPublic)` 合并公共 `SECURITY_OUTBOUND_ALLOWED_HOSTS`、`SECURITY_OUTBOUND_ALLOWED_CIDRS` 与场景专属的 `SECURITY_<scope>_ALLOWED_HOSTS`、`SECURITY_<scope>_ALLOWED_CIDRS`，各项使用逗号分隔。域名白名单不能替代私网 CIDR 授权；`Client` 保留同源重定向，拒绝跨源跳转。

生命周期服务按子模块拆分：

| 子模块 | 能力 |
| --- | --- |
| `transport/cron` | 秒级 Cron、动态任务、时区和优雅停止。 |
| `transport/hptimer` | 单次、间隔和 Cron 高精度定时任务。 |
| `transport/keepalive` | 基于 gRPC health check 的保活服务。 |
| `transport/mcp` | MCP Streamable HTTP、Legacy SSE、进程内和 STDIO。 |
| `transport/queue` | 把 `queue.Queue` 适配为 Kratos `transport.Server`。 |
| `transport/sse` | SSE 服务端、客户端、事件流、认证和断线处理。 |

详细配置和生命周期语义见各子模块 README。所有实现都应通过 `Start`/`Stop` 接入同一个 Kratos App，避免重复创建监听端口或后台 goroutine。
