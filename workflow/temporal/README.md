# workflow/temporal

`workflow/temporal` 封装 Temporal Go SDK，提供客户端、Worker、默认消息工作流和可选 OpenTelemetry 追踪。模块保留 Temporal 的 Workflow、Activity、Signal、Query 等原生概念，适合长周期、可恢复、可观测的业务编排。

## 模块能力

- 创建 Temporal 客户端，支持 HostPort 和 Namespace 配置。
- 异步执行工作流，或同步等待结果。
- 发送 Signal、执行 Query、取消工作流、查询执行描述。
- 创建 Worker，并注册 Workflow 和 Activity。
- 提供 `StartSimpleWorker`，快速处理 `[]byte` 消息体。
- 提供默认 `BrokerMessageWorkflow`，把消息体委托给 `ProcessMessage` Activity。
- 通过 `WithTracing()` 为生产和消费路径创建 OpenTelemetry span。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/workflow/temporal@latest
```

## 客户端

```go
client, err := temporal.NewClient(
	temporal.WithClientHostPort("localhost:7233"),
	temporal.WithClientNamespace("default"),
)
if err != nil {
	return err
}
defer func() { _ = client.Close() }()
```

未传配置时默认连接 `localhost:7233` 和 `default` namespace。

## 执行工作流

### 异步执行

```go
runID, err := client.Execute(ctx, []byte(`{"order_id":"A1001"}`), temporal.ExecuteOptions{
	TaskQueue:  "order-task-queue",
	WorkflowID: "order-A1001",
})
if err != nil {
	return err
}
```

`WorkflowFn` 为空时使用内置 `BrokerMessageWorkflow`。该默认工作流会调用名为 `ProcessMessage` 的 Activity。

### 同步执行

```go
result, err := client.ExecuteSync(ctx, []byte(`{"order_id":"A1001"}`), temporal.ExecuteOptions{
	TaskQueue:  "order-task-queue",
	WorkflowID: "order-A1001-sync",
})
```

`ExecuteSync` 等待工作流完成，并把结果读取为 `[]byte`。

### 自定义工作流

```go
runID, err := client.Execute(ctx, order, temporal.ExecuteOptions{
	TaskQueue:  "order-task-queue",
	WorkflowID: "order-A1001",
	WorkflowFn: OrderWorkflow,
})
```

自定义 Workflow 和 Activity 需要在 Worker 上注册。

## Worker

### 简单 Worker

```go
worker, err := client.StartSimpleWorker(ctx, "order-task-queue",
	func(ctx context.Context, body []byte) error {
		return handleOrder(ctx, body)
	},
)
if err != nil {
	return err
}
```

`StartSimpleWorker` 会创建 Worker、注册默认消息处理 Activity 并立即启动。传入的 `ctx` 取消后会自动停止 Worker。

### 完整 Worker

```go
worker, err := client.NewWorker(temporal.WorkerOptions{
	TaskQueue:  "order-task-queue",
	Workflows:  []any{OrderWorkflow},
	Activities: []any{ProcessOrder},
})
if err != nil {
	return err
}
if err := worker.Start(); err != nil {
	return err
}
```

`NewWorker` 会自动注册内置 `BrokerMessageWorkflow`，并额外注册 `WorkerOptions.Workflows` 和 `WorkerOptions.Activities`。

## 工作流控制

| 方法 | 说明 |
|------|------|
| `Signal(ctx, workflowID, runID, signalName, arg)` | 向运行中的工作流发送 Signal |
| `Query(ctx, workflowID, runID, queryType, arg)` | 查询工作流状态 |
| `Cancel(ctx, workflowID, runID)` | 请求取消工作流 |
| `Describe(ctx, workflowID, runID)` | 获取工作流执行描述 |
| `TemporalClient()` | 返回底层 Temporal SDK Client |

## 配置

### ExecuteOptions

| 字段 | 说明 |
|------|------|
| `TaskQueue` | 工作流使用的任务队列 |
| `WorkflowID` | 工作流执行唯一标识 |
| `WorkflowFn` | 工作流函数，空值使用 `BrokerMessageWorkflow` |
| `RunTimeout` | 单次运行超时 |
| `ExecutionTimeout` | 总执行超时，包含重试和 continue-as-new |
| `TaskTimeout` | 单个 Workflow Task 超时 |
| `RetryPolicy` | Temporal 重试策略 |
| `CronSchedule` | Cron 调度表达式 |
| `IDReusePolicy` | WorkflowID 已存在时的复用策略 |

### WorkerOptions

| 字段 | 说明 |
|------|------|
| `TaskQueue` | Worker 监听的任务队列 |
| `Options` | 原生 Temporal Worker 配置 |
| `Workflows` | 额外注册的 Workflow 函数列表 |
| `Activities` | 额外注册的 Activity 函数或结构体列表 |

## OpenTelemetry

调用 `client.WithTracing()` 后，模块会使用 `go.opentelemetry.io/otel` 创建 tracer：

| 路径 | Span 名称 | Span Kind |
|------|-----------|-----------|
| `Execute` / `ExecuteSync` | `temporal-producer` | Producer |
| `ProcessMessage` Activity | `temporal-consumer` | Consumer |

span 属性会包含 `messaging.system=temporal` 和当前 task queue。

## 本地开发

```bash
temporal server start-dev
```

默认 gRPC 地址为 `localhost:7233`，Web UI 地址为 `http://localhost:8233`。

## 参考

- [Temporal Documentation](https://docs.temporal.io/)
- [Temporal Go SDK](https://github.com/temporalio/sdk-go)
