package conductor

import (
	"time"

	"github.com/conductor-sdk/conductor-go/sdk/model"
)

////////////////////////////////////////////////////////////////////////////////
/// 客户端配置
////////////////////////////////////////////////////////////////////////////////

// ClientOptions 配置 Conductor API 客户端。
type ClientOptions struct {
	// ServerURL 是 Conductor Server API 地址，例如 "http://localhost:8080/api"。
	ServerURL string

	// AuthKey 是认证 key，通常用于 Orkes Cloud。
	AuthKey string

	// AuthSecret 是认证 secret，通常用于 Orkes Cloud。
	AuthSecret string
}

// StartWorkflowOptions 表示启动工作流执行时的参数。
type StartWorkflowOptions struct {
	// Name 是工作流定义名称。
	Name string

	// Version 是工作流定义版本，未设置时使用最新版本。
	Version *int32

	// Input 是工作流输入数据。
	Input map[string]interface{}

	// CorrelationID 用于消息关联。
	CorrelationID string

	// Priority 是工作流优先级。
	Priority *int32
}

// WorkerConfig 表示任务 Worker 配置。
type WorkerConfig struct {
	// TaskType 是该 Worker 轮询的任务定义名称。
	TaskType string

	// Concurrency 是并发 Worker 数量，默认为 1。
	Concurrency int

	// PollInterval 是两次轮询请求之间的间隔，默认为 100ms。
	PollInterval time.Duration

	// Domain 是用于任务隔离的可选 domain。
	Domain string
}

////////////////////////////////////////////////////////////////////////////////
/// 默认值
////////////////////////////////////////////////////////////////////////////////

const (
	defaultServerURL    = "http://localhost:8080/api"
	defaultConcurrency  = 1
	defaultPollInterval = 100 * time.Millisecond
)

////////////////////////////////////////////////////////////////////////////////
/// StartWorkflowRequest 转换辅助方法
////////////////////////////////////////////////////////////////////////////////

func toStartWorkflowRequest(opts StartWorkflowOptions) *model.StartWorkflowRequest {
	req := &model.StartWorkflowRequest{
		Name:  opts.Name,
		Input: opts.Input,
	}
	if opts.Version != nil {
		req.Version = *opts.Version
	}
	if opts.CorrelationID != "" {
		req.CorrelationId = opts.CorrelationID
	}
	if opts.Priority != nil {
		req.Priority = *opts.Priority
	}
	return req
}
