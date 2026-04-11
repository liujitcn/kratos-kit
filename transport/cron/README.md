# kratos-kit cron 传输扩展

`transport/cron` 基于 `robfig/cron/v3` 封装了一个可接入 Kratos 生命周期的定时任务服务。它实现了 `transport.Server` 与 `transport.Endpointer` 接口，适合将周期性任务以“服务”的方式纳入应用统一管理。

## 功能概览

- 基于 Kratos `transport.Server` 接口，支持统一启动与停止
- 基于 `robfig/cron/v3`，支持秒级 cron 表达式
- 支持运行时动态添加、移除和统计任务
- 并发安全，可在多协程中调用任务管理方法
- 停止服务时等待已开始执行的任务完成，默认最多等待 10 秒
- 预留 keepalive 能力，可配合服务注册体系暴露端点

## 安装

```bash
go get github.com/liujitcn/kratos-kit/transport/cron
```

## 核心对象

### `Server`

`Server` 是定时任务服务实例，主要能力如下：

| 方法 | 说明 |
| --- | --- |
| `NewServer(opts ...ServerOption)` | 创建服务实例 |
| `Start(ctx context.Context) error` | 启动调度器 |
| `Stop(ctx context.Context) error` | 停止调度器并等待运行中的任务结束 |
| `StartTimerJob(spec string, cmd func()) (cron.EntryID, error)` | 添加并启动任务 |
| `StopTimerJob(entryID cron.EntryID)` | 停止指定任务 |
| `StopAllJobs()` | 停止全部任务 |
| `GetJobCount() int` | 获取当前已注册任务数量 |
| `Endpoint() (*url.URL, error)` | 获取服务端点 |

### `ServerOption`

当前可用选项如下：

| 选项 | 说明 |
| --- | --- |
| `WithEnableKeepAlive(enable bool)` | 是否启用 keepalive 相关能力 |
| `WithGracefullyShutdown(enable bool)` | 是否配置优雅关闭开关 |

## 快速开始

下面示例演示最小可运行方式。对于仅需要本地定时调度的场景，建议显式关闭 keepalive。

```go
package main

import (
	"context"
	"log"
	"time"

	cronTransport "github.com/liujitcn/kratos-kit/transport/cron"
)

func main() {
	ctx := context.Background()

	srv := cronTransport.NewServer(
		cronTransport.WithEnableKeepAlive(false),
	)

	if err := srv.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := srv.Stop(ctx); err != nil {
			log.Printf("stop cron server failed: %v", err)
		}
	}()

	entryID, err := srv.StartTimerJob("*/10 * * * * *", func() {
		log.Println("task run every 10 seconds")
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("job count: %d, entry id: %d", srv.GetJobCount(), entryID)

	time.Sleep(1 * time.Minute)
}
```

如果你希望将它交给 Kratos 应用统一托管，也可以直接作为 `server` 注入：

```go
app := kratos.New(
	kratos.Name("cron-demo"),
	kratos.Server(srv),
)
```

需要注意的是，`StartTimerJob` 只能在 `srv.Start(...)` 成功之后调用；如果交由 `app.Run()` 托管，请在应用启动完成后再注册任务。

## Cron 表达式

当前实现使用带“秒”字段的解析器，推荐使用 6 段表达式：

```text
# 秒 分 时 日 月 周
*/10 * * * * *   每 10 秒执行一次
0 */1 * * * *    每分钟执行一次
0 0 12 * * *     每天 12:00 执行
0 0 9 * * 1-5    工作日 09:00 执行
```

底层解析器同时启用了 `Descriptor`，因此也支持 `@every 10s` 这类描述符写法。

## 常见操作

```go
// 添加任务
entryID, err := srv.StartTimerJob("0 0 12 * * *", func() {
	log.Println("run at 12:00 every day")
})

// 停止单个任务
srv.StopTimerJob(entryID)

// 停止全部任务
srv.StopAllJobs()

// 查看任务数量
count := srv.GetJobCount()
_ = count
```

## 使用约束与注意事项

### 1. 任务注册必须晚于服务启动

如果服务尚未启动，调用 `StartTimerJob` 会返回错误：

```text
cron server not started, please start server first
```

### 2. `StopAllJobs` 只移除调度，不会强行中断已在执行中的任务

已经开始执行的任务仍会继续运行，直到任务函数自然返回。

### 3. keepalive 默认开关为开启，但当前不会自动创建 keepalive 服务实例

也就是说：

- 如果只是本地定时任务，建议显式配置 `WithEnableKeepAlive(false)`
- 如果开启了 keepalive，但没有为 `Server` 注入实际的 keepalive 实例，`Endpoint()` 可能返回错误

### 4. `WithGracefullyShutdown` 选项当前已暴露，但当前版本的 `Stop` 仍按优雅关闭流程执行

也就是说，关闭服务时仍会等待调度器停止并尽量等待运行中的任务结束。

## 适用场景

- 服务内部的周期性清理任务
- 定时统计、对账、同步、补偿任务
- 希望纳入 Kratos 生命周期管理的轻量定时服务
- 不依赖外部调度平台的单服务定时任务场景
