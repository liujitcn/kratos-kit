# workflow

`workflow` 定义工作流引擎插件的最小公共接口，并把具体引擎能力拆到独立 Go 模块中维护。当前包含 Argo Workflows、Conductor、go-workflows 和 Temporal 四类实现。

## 模块定位

工作流引擎和消息队列的抽象不同：工作流强调持久化编排、任务状态、重试、信号、查询和 Worker 调度，因此本目录不复用 `broker` 接口。公共包只保留跨引擎真正一致的生命周期能力，具体启动、查询、信号、取消、日志等能力由各引擎子模块直接暴露。

## 子模块

| 模块 | 适用场景 | 运行形态 | Worker |
|------|----------|----------|--------|
| `workflow/argo` | Kubernetes 上的容器任务编排、DAG、批处理 | 通过 Argo Server REST API 访问集群 | 无本地 Worker |
| `workflow/conductor` | 微服务任务编排、任务队列式 Worker、可视化流程定义 | 连接 Conductor Server | 有 Task Worker |
| `workflow/goworkflows` | 纯 Go 内嵌式持久工作流、轻量部署 | 进程内运行，状态写入 backend | 有本地 Worker |
| `workflow/temporal` | 长周期高可靠工作流、Signal、Query、Activity 编排 | 连接 Temporal Server | 有 Temporal Worker |

## 安装

按实际引擎安装对应模块：

```bash
go get github.com/liujitcn/kratos-kit/workflow@latest
go get github.com/liujitcn/kratos-kit/workflow/argo@latest
go get github.com/liujitcn/kratos-kit/workflow/conductor@latest
go get github.com/liujitcn/kratos-kit/workflow/goworkflows@latest
go get github.com/liujitcn/kratos-kit/workflow/temporal@latest
```

## 公共接口

### Client

`Client` 只抽象所有引擎都具备的关闭能力：

| 方法 | 说明 |
|------|------|
| `Close() error` | 释放客户端连接或底层资源 |

当前 `argo.WorkflowClient`、`conductor.WorkflowClient`、`goworkflows.WorkflowClient`、`temporal.WorkflowClient` 都实现该接口。

### Worker

`Worker` 抽象本地 Worker 的停止和运行状态能力：

| 方法 | 说明 |
|------|------|
| `Stop()` | 通知 Worker 停止轮询或接收新任务 |
| `IsRunning() bool` | 返回 Worker 当前是否仍处于运行状态 |

当前 `conductor.TaskWorker`、`goworkflows.WorkflowWorker`、`temporal.WorkflowWorker` 实现该接口。Argo Workflows 的任务由 Kubernetes 集群调度，本模块没有本地 Worker 概念。

## 选择建议

- 已经使用 Kubernetes，并希望用 YAML/DAG 管理容器任务时，优先选择 `workflow/argo`。
- 需要面向微服务的任务编排、任务类型 Worker 和流程可视化时，优先选择 `workflow/conductor`。
- 希望不额外部署工作流服务，只在 Go 进程内运行并把状态写入数据库时，优先选择 `workflow/goworkflows`。
- 需要成熟的长周期工作流、Activity、Signal、Query、Cron 和强一致执行语义时，优先选择 `workflow/temporal`。

## 文档

- [workflow/argo](argo/README.md)
- [workflow/conductor](conductor/README.md)
- [workflow/goworkflows](goworkflows/README.md)
- [workflow/temporal](temporal/README.md)
