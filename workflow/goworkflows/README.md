# workflow/goworkflows

`workflow/goworkflows` 封装 [go-workflows](https://github.com/cschleiden/go-workflows)，用于在 Go 进程内运行持久化工作流。模块需要调用方提供 backend，工作流实例、任务和执行状态由 backend 保存。

## 模块能力

- 使用指定 backend 创建工作流客户端。
- 创建、取消、发送信号、查询、等待和删除工作流实例。
- 创建同时处理 Workflow 和 Activity 的 Worker。
- 创建仅处理 Workflow 或仅处理 Activity 的专用 Worker。
- 注册工作流函数和 Activity。
- 暴露底层 `backend.Backend` 和 `client.Client`，便于高级场景使用。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/workflow/goworkflows@latest
```

## 客户端

```go
b := sqlite.NewSqliteBackend("workflows.db")

client, err := goworkflows.NewClient(b)
if err != nil {
	return err
}
defer func() { _ = client.Close() }()
```

`NewClient` 要求 backend 非空。`Close` 会关闭底层 backend，因此同一个 backend 同时被多个组件共享时，需要由调用方统一安排生命周期。

## 工作流实例

### 创建实例

```go
instance, err := client.CreateWorkflowInstance(ctx, goworkflows.CreateWorkflowOptions{
	InstanceID: "order-1001",
	Queue:      workflow.QueueDefault,
}, OrderWorkflow, "order-1001")
if err != nil {
	return err
}
```

`InstanceID` 必填；`Queue` 为空时使用 go-workflows 默认队列。

### 管理实例

| 方法 | 说明 |
|------|------|
| `CancelWorkflowInstance(ctx, instance)` | 取消正在运行的实例 |
| `SignalWorkflow(ctx, instanceID, name, arg)` | 向运行中的实例发送信号 |
| `GetWorkflowInstanceState(ctx, instance)` | 查询实例状态 |
| `WaitForWorkflowInstance(ctx, instance, timeout)` | 等待实例完成，`timeout <= 0` 时使用默认 `20s` |
| `RemoveWorkflowInstance(ctx, instance)` | 删除已完成实例 |
| `RemoveWorkflowInstances(ctx, opts...)` | 批量删除已完成实例 |

## Worker

### 创建 Worker

```go
worker, err := goworkflows.NewWorker(b, &goworkflows.WorkerOptions{
	WorkflowQueues: []workflow.Queue{workflow.QueueDefault},
	ActivityQueues: []workflow.Queue{workflow.QueueDefault},
})
if err != nil {
	return err
}
```

专用 Worker 构造函数：

| 构造函数 | 处理 Workflow | 处理 Activity |
|----------|---------------|---------------|
| `NewWorker` | 是 | 是 |
| `NewWorkflowOnlyWorker` | 是 | 否 |
| `NewActivityOnlyWorker` | 否 | 是 |

### 注册与启动

```go
if err := worker.RegisterWorkflow(OrderWorkflow); err != nil {
	return err
}
if err := worker.RegisterActivity(ProcessOrder); err != nil {
	return err
}
if err := worker.Start(ctx); err != nil {
	return err
}
```

`Start(ctx)` 会在后台启动 Worker，取消传入的 context 会停止后续轮询。需要优雅等待正在执行的任务完成时，先调用 `Stop()`，再调用 `WaitForCompletion()`。

## 配置

### CreateWorkflowOptions

| 字段 | 说明 |
|------|------|
| `InstanceID` | 工作流实例唯一标识，必填 |
| `Queue` | 创建实例使用的队列，空值表示默认队列 |

### WorkerOptions

| 字段 | 说明 |
|------|------|
| `WorkflowPollers` | 工作流任务轮询器数量 |
| `MaxParallelWorkflowTasks` | 并发工作流任务上限，`0` 表示不额外限制 |
| `WorkflowHeartbeatInterval` | 工作流任务心跳间隔 |
| `WorkflowPollingInterval` | 工作流任务轮询间隔 |
| `WorkflowExecutorCacheSize` | 工作流执行器缓存最大数量 |
| `WorkflowExecutorCacheTTL` | 工作流执行器缓存最大存活时间 |
| `WorkflowQueues` | Worker 监听的工作流任务队列 |
| `ActivityPollers` | Activity 任务轮询器数量 |
| `MaxParallelActivityTasks` | 并发 Activity 任务上限，`0` 表示不额外限制 |
| `ActivityHeartbeatInterval` | Activity 任务心跳间隔 |
| `ActivityPollingInterval` | Activity 任务轮询间隔 |
| `ActivityQueues` | Worker 监听的 Activity 任务队列 |
| `SingleWorkerMode` | 是否启用单 Worker 场景优化 |

未设置的 `WorkerOptions` 字段会沿用 go-workflows 上游默认值。

## Worker 方法

| 方法 | 说明 |
|------|------|
| `RegisterWorkflow(wf)` | 注册工作流函数 |
| `RegisterActivity(a)` | 注册 Activity 函数或结构体 |
| `Start(ctx)` | 启动 Worker |
| `Stop()` | 取消内部 context，停止后续轮询 |
| `WaitForCompletion()` | 等待正在执行的任务完成 |
| `IsRunning()` | 返回 Worker 当前是否处于运行状态 |

## 参考

- [go-workflows GitHub](https://github.com/cschleiden/go-workflows)
- [go-workflows Backends](https://github.com/cschleiden/go-workflows#backends)
