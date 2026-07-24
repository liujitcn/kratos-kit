# workflow/argo

`workflow/argo` 是 Argo Workflows 的轻量封装，通过 Argo Server REST API 操作 Kubernetes 中的 Workflow 资源。模块没有引入 Argo 官方 Go SDK 或 `client-go`，适合只需要提交、查询、控制和读取日志的场景。

## 模块能力

- 创建并关闭 Argo REST 客户端。
- 提交 `Workflow`，支持命名空间覆盖、服务端 dry-run 和 `key=value` 参数追加。
- 查询、列表、删除 Workflow。
- 挂起、恢复、终止、重试、重新提交和停止 Workflow。
- 获取整个 Workflow 或指定 Pod 的日志。
- 提供轻量的 Argo Workflow JSON 结构体，覆盖 container、script、DAG、steps 和参数等常见字段。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/workflow/argo@latest
```

## 客户端

```go
client, err := argo.NewClient(argo.ClientOptions{
	ServerURL:         "https://localhost:2746",
	Namespace:         "default",
	Token:             "your-token",
	InsecureSkipVerify: true,
})
if err != nil {
	return err
}
defer func() { _ = client.Close() }()
```

`Token` 只填写 token 原文，模块会自动设置 `Authorization: Bearer <token>`。`InsecureSkipVerify` 仅建议本地开发或测试环境使用。

## Workflow 操作

### 提交

`SubmitWorkflow(ctx, wf, opts)` 会把 `Workflow` 包装为 Argo Server 所需的提交请求。`SubmitOptions.Parameters` 会追加到 `workflow.spec.arguments.parameters`，不会修改调用方传入的 `Workflow` 对象。

```go
wf, err := client.SubmitWorkflow(ctx, &argo.Workflow{
	APIVersion: "argoproj.io/v1alpha1",
	Kind:       "Workflow",
	Metadata: argo.ObjectMeta{
		GenerateName: "hello-",
	},
	Spec: argo.WorkflowSpec{
		Entrypoint: "main",
		Templates: []argo.Template{
			{
				Name: "main",
				Container: &argo.Container{
					Image:   "alpine:3.20",
					Command: []string{"sh", "-c"},
					Args:    []string{"echo hello"},
				},
			},
		},
	},
}, &argo.SubmitOptions{
	Parameters: []string{"message=hello"},
})
if err != nil {
	return err
}
```

### 查询与列表

```go
wf, err := client.GetWorkflow(ctx, "hello-abcde", "")
list, err := client.ListWorkflows(ctx, &argo.ListOptions{
	LabelSelector: "workflows.argoproj.io/completed=true",
	Limit:         20,
})
```

第三个命名空间参数为空时使用 `ClientOptions.Namespace`。`ListOptions.Namespace` 可覆盖客户端默认命名空间。

### 生命周期

| 方法 | 说明 |
|------|------|
| `SuspendWorkflow(ctx, name, namespace)` | 挂起运行中的 Workflow |
| `ResumeWorkflow(ctx, name, namespace)` | 恢复已挂起的 Workflow |
| `TerminateWorkflow(ctx, name, namespace)` | 终止运行中的 Workflow |
| `RetryWorkflow(ctx, name, namespace)` | 重试失败的 Workflow，并返回更新后的资源 |
| `ResubmitWorkflow(ctx, name, namespace)` | 基于已有 Workflow 重新提交一次执行 |
| `StopWorkflow(ctx, name, namespace, message)` | 停止 Workflow，可附带停止原因 |
| `DeleteWorkflow(ctx, name, namespace)` | 删除 Workflow |

### 日志

```go
logs, err := client.GetWorkflowLogs(ctx, "hello-abcde", "", "")
podLogs, err := client.GetWorkflowLogs(ctx, "hello-abcde", "", "hello-abcde-main")
```

`podName` 为空时读取 Workflow 级日志；非空时读取指定 Pod 日志。

## 配置

### ClientOptions

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `ServerURL` | Argo Server API 地址 | `https://localhost:2746` |
| `Namespace` | 默认 Kubernetes 命名空间 | `default` |
| `Token` | Bearer token 原文 | 空 |
| `InsecureSkipVerify` | 是否跳过 TLS 证书校验 | `false` |

### SubmitOptions

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `Namespace` | 提交目标命名空间，覆盖客户端默认值 | 空 |
| `ServerDryRun` | 只在服务端校验，不实际创建 Workflow | `false` |
| `Parameters` | `key=value` 格式参数，追加到 Workflow arguments | 空 |

### ListOptions

| 字段 | 说明 |
|------|------|
| `Namespace` | 查询目标命名空间，覆盖客户端默认值 |
| `LabelSelector` | Kubernetes label selector |
| `FieldSelector` | Kubernetes field selector |
| `Limit` | 返回数量上限 |
| `Offset` | 分页偏移 |

## 类型与状态

`Phase` 取值包括 `Pending`、`Running`、`Succeeded`、`Failed`、`Error`。可调用 `phase.IsTerminal()` 判断是否已经到达终态，其中 `Succeeded`、`Failed`、`Error` 为终态。

## 参考

- [Argo Workflows Documentation](https://argoproj.github.io/argo-workflows/)
- [Argo Workflows REST API](https://argo-workflows.readthedocs.io/en/latest/rest-api/)
