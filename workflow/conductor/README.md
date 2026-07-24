# workflow/conductor

`workflow/conductor` 封装 Conductor Go SDK，保留 Conductor 的 Workflow、Task、Worker 等原生概念。模块适合需要流程定义、任务类型 Worker、任务重试和流程可视化的微服务编排场景。

## 模块能力

- 创建 Conductor API 客户端，支持显式配置或环境变量配置。
- 异步启动工作流，或同步等待工作流完成、等待指定任务完成。
- 查询工作流状态，监听执行结果。
- 暂停、恢复、终止、重试、重启工作流。
- 启动任务 Worker，支持并发数、轮询间隔和 domain 配置。
- 暴露底层 `APIClient` 和 `WorkflowExecutor`，便于调用 SDK 的高级能力。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/workflow/conductor@latest
```

## 客户端

```go
client, err := conductor.NewClient(conductor.ClientOptions{
	ServerURL:  "http://localhost:8080/api",
	AuthKey:    "",
	AuthSecret: "",
})
if err != nil {
	return err
}
defer func() { _ = client.Close() }()
```

也可以使用环境变量创建客户端：

```go
client, err := conductor.NewClientFromEnv()
```

`NewClientFromEnv` 由 Conductor SDK 读取环境变量，常用变量包括 `CONDUCTOR_SERVER_URL`、`CONDUCTOR_AUTH_KEY`、`CONDUCTOR_AUTH_SECRET`。

## 工作流执行

### 启动工作流

```go
workflowID, err := client.StartWorkflow(ctx, conductor.StartWorkflowOptions{
	Name: "order_flow",
	Input: map[string]interface{}{
		"order_id": "A1001",
	},
})
if err != nil {
	return err
}
```

`StartWorkflow` 返回 Conductor 工作流实例 ID。`Version` 和 `Priority` 使用指针类型，未设置时交由 Conductor 使用默认行为。

### 同步执行

```go
run, err := client.StartWorkflowSync(ctx, conductor.StartWorkflowOptions{
	Name: "order_flow",
	Input: map[string]interface{}{
		"order_id": "A1001",
	},
}, "")
```

第三个参数 `waitUntilTask` 为空时等待工作流完成；非空时等待指定任务完成。

### 查询和控制

| 方法 | 说明 |
|------|------|
| `GetWorkflow(ctx, workflowID, includeTasks)` | 查询工作流状态，可选择是否包含任务列表 |
| `MonitorExecution(workflowID)` | 返回执行结果通道，用于异步监听完成事件 |
| `Pause(ctx, workflowID)` | 暂停工作流 |
| `Resume(ctx, workflowID)` | 恢复工作流 |
| `Terminate(ctx, workflowID, reason)` | 终止工作流 |
| `Retry(ctx, workflowID, resumeSubworkflowTasks)` | 从失败任务开始重试 |
| `Restart(ctx, workflowID, useLatestDef)` | 从头重启已到达终态的工作流 |

## 任务 Worker

`TaskHandler` 是 Conductor SDK 的 `model.ExecuteTaskFunction`。可以用最简方式启动 Worker：

```go
worker, err := client.StartWorker("ship_order", handler, 3, 200*time.Millisecond)
if err != nil {
	return err
}
defer worker.Stop()
```

也可以使用完整配置：

```go
worker, err := client.StartWorkerWithConfig(conductor.WorkerConfig{
	TaskType:     "ship_order",
	Concurrency:  3,
	PollInterval: 200 * time.Millisecond,
	Domain:       "default",
}, handler)
```

`Concurrency <= 0` 时使用默认值 `1`；`PollInterval <= 0` 时使用默认值 `100ms`。

## 配置

### ClientOptions

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `ServerURL` | Conductor Server API 地址 | `http://localhost:8080/api` |
| `AuthKey` | 认证 key，通常用于 Orkes Cloud | 空 |
| `AuthSecret` | 认证 secret，通常用于 Orkes Cloud | 空 |

### StartWorkflowOptions

| 字段 | 说明 |
|------|------|
| `Name` | 工作流定义名称 |
| `Version` | 工作流定义版本，空值使用 Conductor 默认行为 |
| `Input` | 工作流输入数据 |
| `CorrelationID` | 关联 ID |
| `Priority` | 工作流优先级 |

### WorkerConfig

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `TaskType` | Worker 轮询的任务类型 | 空 |
| `Concurrency` | 并发 Worker 数量 | `1` |
| `PollInterval` | 两次轮询之间的间隔 | `100ms` |
| `Domain` | 任务隔离 domain | 空 |

## Worker 方法

| 方法 | 说明 |
|------|------|
| `Stop()` | 停止当前任务类型的底层轮询 |
| `WaitForCompletion()` | 等待底层 Worker 完成退出 |
| `TaskType()` | 返回当前 Worker 轮询的任务类型 |
| `IsRunning()` | 返回 Worker 是否仍处于运行状态 |

## 参考

- [Conductor Documentation](https://conductor-oss.github.io/conductor/)
- [Conductor Go SDK](https://github.com/conductor-oss/go-sdk)
