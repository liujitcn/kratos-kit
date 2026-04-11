# kratos-kit hptimer 传输扩展

`transport/hptimer` 提供了一个可接入 Kratos 生命周期的高精度定时任务服务。底层使用最小堆配合单个 `time.Timer` 调度任务，适合需要毫秒级触发、动态增删任务的场景。

## 功能概览

- 基于 Kratos `transport.Server` 接口，支持统一启动与停止
- 支持单次任务、固定间隔循环任务、Cron 表达式任务
- 基于最小堆调度，适合大量动态任务的增删与触发
- 支持 `TimerObserver`，可将“调度”和“执行”解耦
- 停止服务时会取消任务上下文，并等待调度引擎退出
- 预留 keepalive 能力，可配合服务注册体系暴露端点

## 安装

```bash
go get github.com/liujitcn/kratos-kit/transport/hptimer
```

## 核心对象

### `Server`

`Server` 是对高精度定时引擎的服务化封装，主要方法如下：

| 方法 | 说明 |
| --- | --- |
| `NewServer(opts ...ServerOption)` | 创建服务实例 |
| `Start(ctx context.Context) error` | 启动高精度定时引擎 |
| `Stop(ctx context.Context) error` | 停止引擎并取消任务上下文 |
| `AddTask(task *TimerTask) TimerTaskID` | 添加任务，失败时返回空字符串 |
| `RemoveTask(taskID TimerTaskID) bool` | 删除任务，成功返回 `true` |
| `Endpoint() (*url.URL, error)` | 获取服务端点 |

### `ServerOption`

当前可用选项如下：

| 选项 | 说明 |
| --- | --- |
| `WithEnableKeepAlive(enable bool)` | 是否启用 keepalive 相关能力 |
| `WithGracefullyShutdown(enable bool)` | 是否配置优雅关闭开关 |
| `WithTimerObserver(observer TimerObserver)` | 设置任务触发观察者 |

### `TimerTask`

`TimerTask` 是任务定义结构，核心字段如下：

| 字段 | 说明 |
| --- | --- |
| `ID` | 任务唯一标识，不能为空 |
| `At` | 首次触发时间 |
| `Interval` | 循环间隔；大于 `0` 时表示循环任务 |
| `Cron` | Cron 表达式；非空时按表达式计算下次触发时间 |
| `Data` | 可选业务负载 |
| `Priority` | 优先级字段，供业务侧扩展使用 |
| `Callback` | 任务触发时执行的回调 |
| `Ctx` | 任务上下文，删除任务或停止服务时会被取消 |

任务也可以通过 `NewTimerTask(...)` 和可选参数创建，例如：

- `WithInterval(d time.Duration)`
- `WithCron(expr string)`
- `WithData(v any)`
- `WithCallback(cb func(ctx context.Context) error)`
- `WithPriority(p Priority)`
- `WithContext(parent context.Context)`

## 快速开始

对于仅需要本地定时调度的场景，建议显式关闭 keepalive。

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/liujitcn/kratos-kit/transport/hptimer"
)

func main() {
	ctx := context.Background()

	srv := hptimer.NewServer(
		hptimer.WithEnableKeepAlive(false),
	)

	if err := srv.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := srv.Stop(ctx); err != nil {
			log.Printf("stop hptimer server failed: %v", err)
		}
	}()

	taskID := srv.AddTask(hptimer.NewTimerTask(
		"task-once",
		time.Now().Add(200*time.Millisecond),
		hptimer.WithCallback(func(ctx context.Context) error {
			log.Println("task triggered")
			return nil
		}),
	))
	if taskID == "" {
		log.Fatal("add task failed")
	}

	time.Sleep(1 * time.Second)
}
```

如果你希望将它交给 Kratos 应用统一托管，也可以直接作为 `server` 注入：

```go
app := kratos.New(
	kratos.Name("hptimer-demo"),
	kratos.Server(srv),
)
```

需要注意的是，`AddTask` 只能在 `srv.Start(...)` 成功之后调用。

## 三种任务模式

### 1. 单次任务

```go
taskID := srv.AddTask(hptimer.NewTimerTask(
	"task-once",
	time.Now().Add(500*time.Millisecond),
	hptimer.WithCallback(func(ctx context.Context) error {
		log.Println("run once")
		return nil
	}),
))
```

### 2. 固定间隔循环任务

```go
taskID := srv.AddTask(hptimer.NewTimerTask(
	"task-interval",
	time.Now().Add(100*time.Millisecond),
	hptimer.WithInterval(1*time.Second),
	hptimer.WithCallback(func(ctx context.Context) error {
		log.Println("run every 1 second")
		return nil
	}),
))
```

### 3. Cron 任务

当 `At` 为零值且设置了 `Cron` 时，首次触发时间会在 `AddTask` 时自动计算：

```go
taskID := srv.AddTask(hptimer.NewTimerTask(
	"task-cron",
	time.Time{},
	hptimer.WithCron("*/1 * * * * *"),
	hptimer.WithCallback(func(ctx context.Context) error {
		log.Println("run every second")
		return nil
	}),
))
```

当前实现基于 `github.com/gorhill/cronexpr` 解析表达式，支持带秒字段的写法，例如：

```text
*/1 * * * * *    每秒执行一次
0 */1 * * * *    每分钟执行一次
0 0 9 * * 1-5    工作日 09:00 执行
```

## 任务删除

```go
removed := srv.RemoveTask("task-once")
if !removed {
	log.Println("task not found or already triggered")
}
```

对于已经触发并完成的单次任务，`RemoveTask` 会返回 `false`。

## 使用 `TimerObserver`

如果你不希望在调度器内部直接执行业务逻辑，可以通过 `TimerObserver` 接收任务触发事件，再由业务层决定如何处理：

```go
type observer struct{}

func (o *observer) OnTimerTrigger(task *hptimer.TimerTask) {
	log.Printf("task triggered: %s", task.ID)
	if task.Callback != nil {
		_ = task.Callback(task.Ctx)
	}
}

srv := hptimer.NewServer(
	hptimer.WithEnableKeepAlive(false),
	hptimer.WithTimerObserver(&observer{}),
)
```

如果没有显式设置观察者，默认行为是直接执行任务的 `Callback`。

## 使用约束与注意事项

### 1. 添加任务前必须先启动服务

如果服务未启动，`AddTask` 会直接返回空字符串。

### 2. 任务 ID 必须唯一

重复添加相同 `ID` 的任务会失败，并返回空字符串。

### 3. 循环任务通过重新入堆实现

也就是说：

- `Interval > 0` 时会按固定间隔重新调度
- `Cron != ""` 时会按表达式计算下一次触发时间
- `Interval` 与 `Cron` 同时设置时，当前实现会优先按 `Interval` 处理循环

### 4. 删除任务或停止服务会取消任务上下文

如果你的回调逻辑依赖取消信号，可以通过 `ctx.Done()` 感知任务被移除或服务关闭。

### 5. keepalive 默认开关为开启，但当前不会自动创建 keepalive 服务实例

也就是说：

- 如果只是本地定时任务，建议显式配置 `WithEnableKeepAlive(false)`
- 如果开启了 keepalive，但没有为 `Server` 注入实际的 keepalive 实例，`Endpoint()` 可能返回错误

### 6. `WithGracefullyShutdown` 选项当前已暴露，但当前版本的 `Stop` 仍按优雅关闭流程执行

关闭服务时仍会等待调度引擎退出，并取消已有任务上下文。

## 基准测试

仓库中包含基准测试，可在模块目录执行：

```bash
go test -bench . -run ^$ ./...
```

当前基准函数包括：

- `BenchmarkHighPrecisionTimer_SingleTask`
- `BenchmarkHighPrecisionTimer_BatchTasks`

## 适用场景

- 毫秒级超时控制、延时触发、重试调度
- 大量动态增删的定时任务场景
- 需要将定时能力纳入 Kratos 生命周期管理的服务
- 相比传统 cron 更关注触发精度和资源占用的场景
