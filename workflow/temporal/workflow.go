package temporal

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

const (
	defaultActivityName    = "ProcessMessage"
	defaultActivityTimeout = time.Minute * 5
)

// BrokerMessageWorkflow 是默认消息工作流。
// 它接收消息体，并委托已注册的 ProcessMessage Activity 处理。
// 复杂编排场景可通过 WorkerOptions.Workflows 注册自定义工作流。
func BrokerMessageWorkflow(ctx workflow.Context, body []byte) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: defaultActivityTimeout,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	return workflow.ExecuteActivity(ctx, defaultActivityName, body).Get(ctx, nil)
}
