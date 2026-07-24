package goworkflows

import (
	"time"

	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

////////////////////////////////////////////////////////////////////////////////
/// 创建工作流配置
////////////////////////////////////////////////////////////////////////////////

// CreateWorkflowOptions 表示创建工作流实例时的参数。
type CreateWorkflowOptions struct {
	// InstanceID 是工作流实例唯一标识，必填。
	InstanceID string

	// Queue 是创建工作流实例时使用的队列，空值表示使用 workflow.QueueDefault。
	Queue workflow.Queue
}

////////////////////////////////////////////////////////////////////////////////
/// Worker 配置
////////////////////////////////////////////////////////////////////////////////

// WorkerOptions 表示 Worker 配置。
type WorkerOptions struct {
	// WorkflowPollers 是工作流任务轮询器数量，默认值由上游库决定。
	WorkflowPollers int

	// MaxParallelWorkflowTasks 是并发工作流任务上限，0 表示不额外限制。
	MaxParallelWorkflowTasks int

	// WorkflowHeartbeatInterval 是工作流任务心跳间隔。
	WorkflowHeartbeatInterval time.Duration

	// WorkflowPollingInterval 是工作流任务轮询间隔。
	WorkflowPollingInterval time.Duration

	// WorkflowExecutorCacheSize 是工作流执行器缓存最大数量。
	WorkflowExecutorCacheSize int

	// WorkflowExecutorCacheTTL 是工作流执行器缓存最大存活时间。
	WorkflowExecutorCacheTTL time.Duration

	// WorkflowQueues 是 Worker 监听的工作流任务队列。
	WorkflowQueues []workflow.Queue

	// ActivityPollers 是 Activity 任务轮询器数量，默认值由上游库决定。
	ActivityPollers int

	// MaxParallelActivityTasks 是并发 Activity 任务上限，0 表示不额外限制。
	MaxParallelActivityTasks int

	// ActivityHeartbeatInterval 是 Activity 任务心跳间隔。
	ActivityHeartbeatInterval time.Duration

	// ActivityPollingInterval 是 Activity 任务轮询间隔。
	ActivityPollingInterval time.Duration

	// ActivityQueues 是 Worker 监听的 Activity 任务队列。
	ActivityQueues []workflow.Queue

	// SingleWorkerMode 启用单 Worker 场景优化。
	SingleWorkerMode bool
}

////////////////////////////////////////////////////////////////////////////////
/// 默认值
////////////////////////////////////////////////////////////////////////////////

const (
	defaultWaitTimeout = 20 * time.Second
)

// toWorkerOptions 将本模块配置转换为上游 worker.Options。
func (wo *WorkerOptions) toWorkerOptions() worker.Options {
	opts := worker.DefaultOptions

	if wo == nil {
		return opts
	}

	if wo.WorkflowPollers > 0 {
		opts.WorkflowPollers = wo.WorkflowPollers
	}
	if wo.MaxParallelWorkflowTasks > 0 {
		opts.MaxParallelWorkflowTasks = wo.MaxParallelWorkflowTasks
	}
	if wo.WorkflowHeartbeatInterval > 0 {
		opts.WorkflowHeartbeatInterval = wo.WorkflowHeartbeatInterval
	}
	if wo.WorkflowPollingInterval > 0 {
		opts.WorkflowPollingInterval = wo.WorkflowPollingInterval
	}
	if wo.WorkflowExecutorCacheSize > 0 {
		opts.WorkflowExecutorCacheSize = wo.WorkflowExecutorCacheSize
	}
	if wo.WorkflowExecutorCacheTTL > 0 {
		opts.WorkflowExecutorCacheTTL = wo.WorkflowExecutorCacheTTL
	}
	if len(wo.WorkflowQueues) > 0 {
		opts.WorkflowQueues = wo.WorkflowQueues
	}

	if wo.ActivityPollers > 0 {
		opts.ActivityPollers = wo.ActivityPollers
	}
	if wo.MaxParallelActivityTasks > 0 {
		opts.MaxParallelActivityTasks = wo.MaxParallelActivityTasks
	}
	if wo.ActivityHeartbeatInterval > 0 {
		opts.ActivityHeartbeatInterval = wo.ActivityHeartbeatInterval
	}
	if wo.ActivityPollingInterval > 0 {
		opts.ActivityPollingInterval = wo.ActivityPollingInterval
	}
	if len(wo.ActivityQueues) > 0 {
		opts.ActivityQueues = wo.ActivityQueues
	}

	opts.SingleWorkerMode = wo.SingleWorkerMode

	return opts
}
