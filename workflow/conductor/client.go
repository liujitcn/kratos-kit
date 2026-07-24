package conductor

import (
	"context"
	"fmt"
	"sync"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/conductor-sdk/conductor-go/sdk/workflow/executor"
	"github.com/go-kratos/kratos/v3/log"
)

// WorkflowClient 提供 Conductor 工作流操作的高层封装。
type WorkflowClient struct {
	mu               sync.RWMutex
	apiClient        *client.APIClient
	workflowExecutor *executor.WorkflowExecutor
	workflowClient   client.WorkflowClient
	options          ClientOptions
	running          bool
}

// NewClient 创建并初始化 Conductor 客户端。
func NewClient(opts ClientOptions) (*WorkflowClient, error) {
	if opts.ServerURL == "" {
		opts.ServerURL = defaultServerURL
	}

	var authSettings *settings.AuthenticationSettings
	if opts.AuthKey != "" && opts.AuthSecret != "" {
		authSettings = settings.NewAuthenticationSettings(opts.AuthKey, opts.AuthSecret)
	}

	httpSettings := settings.NewHttpSettings(opts.ServerURL)

	var apiClient *client.APIClient
	if authSettings != nil {
		apiClient = client.NewAPIClient(authSettings, httpSettings)
	} else {
		apiClient = client.NewAPIClient(nil, httpSettings)
	}

	workflowExecutor := executor.NewWorkflowExecutor(apiClient)
	workflowClient := client.NewWorkflowClient(apiClient)

	log.Info(fmt.Sprintf("connected to Conductor server at %s", opts.ServerURL))

	return &WorkflowClient{
		apiClient:        apiClient,
		workflowExecutor: workflowExecutor,
		workflowClient:   workflowClient,
		options:          opts,
		running:          true,
	}, nil
}

// NewClientFromEnv 基于环境变量创建 Conductor 客户端。
// 读取的变量包括 CONDUCTOR_SERVER_URL、CONDUCTOR_AUTH_KEY、CONDUCTOR_AUTH_SECRET。
func NewClientFromEnv() (*WorkflowClient, error) {
	apiClient := client.NewAPIClientFromEnv()

	workflowExecutor := executor.NewWorkflowExecutor(apiClient)
	workflowClient := client.NewWorkflowClient(apiClient)

	log.Info("connected to Conductor server from environment")

	return &WorkflowClient{
		apiClient:        apiClient,
		workflowExecutor: workflowExecutor,
		workflowClient:   workflowClient,
		options:          ClientOptions{},
		running:          true,
	}, nil
}

// APIClient 返回底层 Conductor API 客户端，供高级场景使用。
func (wc *WorkflowClient) APIClient() *client.APIClient {
	return wc.apiClient
}

// WorkflowExecutor 返回底层工作流执行器。
func (wc *WorkflowClient) WorkflowExecutor() *executor.WorkflowExecutor {
	return wc.workflowExecutor
}

// Close 标记客户端停止运行并释放相关资源。
func (wc *WorkflowClient) Close() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.running = false
	log.Info("disconnected from Conductor server")
	return nil
}

// StartWorkflow 异步启动一次工作流执行。
// 返回 Conductor 工作流实例 ID。
func (wc *WorkflowClient) StartWorkflow(ctx context.Context, opts StartWorkflowOptions) (string, error) {
	if wc.workflowExecutor == nil {
		return "", fmt.Errorf("workflow executor is nil")
	}

	req := toStartWorkflowRequest(opts)
	var id string
	var err error
	id, err = wc.workflowExecutor.StartWorkflowWithContext(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("started workflow %s with id: %s", opts.Name, id))
	return id, nil
}

// StartWorkflowSync 启动工作流，并阻塞到工作流完成或指定任务完成。
// 返回 Conductor 工作流执行结果。
func (wc *WorkflowClient) StartWorkflowSync(ctx context.Context, opts StartWorkflowOptions, waitUntilTask string) (*model.WorkflowRun, error) {
	if wc.workflowExecutor == nil {
		return nil, fmt.Errorf("workflow executor is nil")
	}

	req := toStartWorkflowRequest(opts)
	var run *model.WorkflowRun
	var err error
	run, err = wc.workflowExecutor.ExecuteWorkflowWithContext(ctx, req, waitUntilTask)
	if err != nil {
		return nil, fmt.Errorf("execute workflow error: %w", err)
	}

	return run, nil
}

// MonitorExecution 返回用于接收工作流完成结果的通道。
func (wc *WorkflowClient) MonitorExecution(workflowID string) (executor.WorkflowExecutionChannel, error) {
	if wc.workflowExecutor == nil {
		return nil, fmt.Errorf("workflow executor is nil")
	}

	return wc.workflowExecutor.MonitorExecution(workflowID)
}

// GetWorkflow 获取工作流执行的当前状态。
func (wc *WorkflowClient) GetWorkflow(ctx context.Context, workflowID string, includeTasks bool) (*model.Workflow, error) {
	if wc.workflowClient == nil {
		return nil, fmt.Errorf("workflow client is nil")
	}

	opts := &client.WorkflowResourceApiGetExecutionStatusOpts{
		IncludeTasks: optional.NewBool(includeTasks),
	}
	var workflow model.Workflow
	var err error
	workflow, _, err = wc.workflowClient.GetExecutionStatus(ctx, workflowID, opts)
	if err != nil {
		return nil, fmt.Errorf("get workflow error: %w", err)
	}

	return &workflow, nil
}

// Terminate 终止正在运行的工作流。
func (wc *WorkflowClient) Terminate(ctx context.Context, workflowID, reason string) error {
	if wc.workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}

	terminateOpts := &client.WorkflowResourceApiTerminateOpts{
		Reason:                 optional.NewString(reason),
		TriggerFailureWorkflow: optional.NewBool(false),
	}
	var err error
	_, err = wc.workflowClient.Terminate(ctx, workflowID, terminateOpts)
	if err != nil {
		return fmt.Errorf("terminate workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("terminated workflow %s", workflowID))
	return nil
}

// Pause 暂停正在执行的工作流。
func (wc *WorkflowClient) Pause(ctx context.Context, workflowID string) error {
	if wc.workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}

	var err error
	_, err = wc.workflowClient.Pause(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("pause workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("paused workflow %s", workflowID))
	return nil
}

// Resume 恢复已暂停的工作流。
func (wc *WorkflowClient) Resume(ctx context.Context, workflowID string) error {
	if wc.workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}

	var err error
	_, err = wc.workflowClient.Resume(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("resume workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("resumed workflow %s", workflowID))
	return nil
}

// Restart 从头重新启动已到达终态的工作流。
func (wc *WorkflowClient) Restart(ctx context.Context, workflowID string, useLatestDef bool) error {
	if wc.workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}

	restartOpts := &client.WorkflowResourceApiRestartOpts{
		UseLatestDefinitions: optional.NewBool(useLatestDef),
	}
	var err error
	_, err = wc.workflowClient.Restart(ctx, workflowID, restartOpts)
	if err != nil {
		return fmt.Errorf("restart workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("restarted workflow %s", workflowID))
	return nil
}

// Retry 从最后失败的任务开始重试工作流。
func (wc *WorkflowClient) Retry(ctx context.Context, workflowID string, resumeSubworkflowTasks bool) error {
	if wc.workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}

	retryOpts := &client.WorkflowResourceApiRetryOpts{
		ResumeSubworkflowTasks: optional.NewBool(resumeSubworkflowTasks),
	}
	var err error
	_, err = wc.workflowClient.Retry(ctx, workflowID, retryOpts)
	if err != nil {
		return fmt.Errorf("retry workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("retried workflow %s", workflowID))
	return nil
}
