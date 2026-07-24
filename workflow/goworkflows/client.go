package goworkflows

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/go-kratos/kratos/v3/log"
)

// WorkflowClient 提供 go-workflows 工作流实例管理的高层封装。
type WorkflowClient struct {
	mu      sync.RWMutex
	backend backend.Backend
	client  *client.Client
}

// NewClient 使用指定 backend 创建工作流客户端。
func NewClient(b backend.Backend) (*WorkflowClient, error) {
	if b == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	c := client.New(b)

	log.Info("created workflow client")

	return &WorkflowClient{
		backend: b,
		client:  c,
	}, nil
}

// Backend 返回底层 backend。
func (wc *WorkflowClient) Backend() backend.Backend {
	return wc.backend
}

// InnerClient 返回底层 go-workflows 客户端，供高级场景使用。
func (wc *WorkflowClient) InnerClient() *client.Client {
	return wc.client
}

// Close 关闭底层 backend 资源。
func (wc *WorkflowClient) Close() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	if wc.backend != nil {
		log.Info("closing workflow client and backend")
		return wc.backend.Close()
	}
	return nil
}

// CreateWorkflowInstance 创建新的工作流实例。
// 工作流函数与参数会直接透传给 go-workflows 客户端。
func (wc *WorkflowClient) CreateWorkflowInstance(ctx context.Context, opts CreateWorkflowOptions, wf workflow.Workflow, args ...any) (*workflow.Instance, error) {
	if wc.client == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if opts.InstanceID == "" {
		return nil, fmt.Errorf("InstanceID must be set in CreateWorkflowOptions")
	}

	clientOpts := client.WorkflowInstanceOptions{
		InstanceID: opts.InstanceID,
		Queue:      opts.Queue,
	}

	instance, err := wc.client.CreateWorkflowInstance(ctx, clientOpts, wf, args...)
	if err != nil {
		return nil, fmt.Errorf("create workflow instance error: %w", err)
	}

	log.Info(fmt.Sprintf("created workflow instance: %s (execution: %s)", instance.InstanceID, instance.ExecutionID))
	return instance, nil
}

// CancelWorkflowInstance 取消正在运行的工作流实例。
func (wc *WorkflowClient) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance) error {
	if wc.client == nil {
		return fmt.Errorf("client is nil")
	}

	err := wc.client.CancelWorkflowInstance(ctx, instance)
	if err != nil {
		return fmt.Errorf("cancel workflow instance error: %w", err)
	}

	log.Info(fmt.Sprintf("cancelled workflow instance: %s", instance.InstanceID))
	return nil
}

// SignalWorkflow 向正在运行的工作流实例发送信号。
func (wc *WorkflowClient) SignalWorkflow(ctx context.Context, instanceID string, name string, arg any) error {
	if wc.client == nil {
		return fmt.Errorf("client is nil")
	}

	err := wc.client.SignalWorkflow(ctx, instanceID, name, arg)
	if err != nil {
		return fmt.Errorf("signal workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("signaled workflow instance %s with signal %s", instanceID, name))
	return nil
}

// GetWorkflowInstanceState 返回指定工作流实例的当前状态。
func (wc *WorkflowClient) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error) {
	if wc.client == nil {
		return core.WorkflowInstanceStateActive, fmt.Errorf("client is nil")
	}

	state, err := wc.client.GetWorkflowInstanceState(ctx, instance)
	if err != nil {
		return core.WorkflowInstanceStateActive, fmt.Errorf("get workflow state error: %w", err)
	}

	return state, nil
}

// WaitForWorkflowInstance 等待指定工作流实例完成。
// timeout 小于等于 0 时使用默认等待超时时间。
func (wc *WorkflowClient) WaitForWorkflowInstance(ctx context.Context, instance *workflow.Instance, timeout time.Duration) error {
	if wc.client == nil {
		return fmt.Errorf("client is nil")
	}

	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}

	err := wc.client.WaitForWorkflowInstance(ctx, instance, timeout)
	if err != nil {
		return fmt.Errorf("wait for workflow instance error: %w", err)
	}

	log.Info(fmt.Sprintf("workflow instance %s finished", instance.InstanceID))
	return nil
}

// RemoveWorkflowInstance 从 backend 删除已完成的工作流实例。
func (wc *WorkflowClient) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	if wc.client == nil {
		return fmt.Errorf("client is nil")
	}

	err := wc.client.RemoveWorkflowInstance(ctx, instance)
	if err != nil {
		return fmt.Errorf("remove workflow instance error: %w", err)
	}

	log.Info(fmt.Sprintf("removed workflow instance: %s", instance.InstanceID))
	return nil
}

// RemoveWorkflowInstances 从 backend 批量删除已完成的工作流实例。
func (wc *WorkflowClient) RemoveWorkflowInstances(ctx context.Context, opts ...backend.RemovalOption) error {
	if wc.client == nil {
		return fmt.Errorf("client is nil")
	}

	err := wc.client.RemoveWorkflowInstances(ctx, opts...)
	if err != nil {
		return fmt.Errorf("remove workflow instances error: %w", err)
	}

	log.Info("removed workflow instances")
	return nil
}
