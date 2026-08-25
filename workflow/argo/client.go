package argo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v3/log"
)

// WorkflowClient 通过 REST API 提供 Argo Workflows 高层操作能力。
type WorkflowClient struct {
	mu      sync.RWMutex
	options ClientOptions
	client  *http.Client
	baseURL string
	running bool
}

// NewClient 创建并初始化 Argo Workflows 客户端。
func NewClient(opts ClientOptions) (*WorkflowClient, error) {
	if opts.ServerURL == "" {
		opts.ServerURL = defaultServerURL
	}
	if opts.Namespace == "" {
		opts.Namespace = defaultNamespace
	}

	baseURL := strings.TrimRight(opts.ServerURL, "/")

	httpClient := &http.Client{}
	if opts.InsecureSkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	log.Info(fmt.Sprintf("connected to Argo Server at %s (namespace: %s)", baseURL, opts.Namespace))

	return &WorkflowClient{
		options: opts,
		client:  httpClient,
		baseURL: baseURL,
		running: true,
	}, nil
}

// Close 关闭空闲连接并标记客户端停止运行。
func (wc *WorkflowClient) Close() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.running = false
	wc.client.CloseIdleConnections()
	log.Info("disconnected from Argo Server")
	return nil
}

////////////////////////////////////////////////////////////////////////////////
/// HTTP 辅助方法
////////////////////////////////////////////////////////////////////////////////

func (wc *WorkflowClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var reader io.Reader
	var err error
	if body != nil {
		var data []byte
		data, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body error: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, method, wc.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if wc.options.Token != "" {
		req.Header.Set("Authorization", "Bearer "+wc.options.Token)
	}

	return req, nil
}

func (wc *WorkflowClient) doRequest(req *http.Request, result interface{}) error {
	var resp *http.Response
	var err error
	resp, err = wc.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	var data []byte
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	if result != nil && len(data) > 0 {
		err = json.Unmarshal(data, result)
		if err != nil {
			return fmt.Errorf("unmarshal response error: %w", err)
		}
	}

	return nil
}

func (wc *WorkflowClient) namespace(optsNamespace string) string {
	if optsNamespace != "" {
		return optsNamespace
	}
	return wc.options.Namespace
}

////////////////////////////////////////////////////////////////////////////////
/// Workflow 基础操作
////////////////////////////////////////////////////////////////////////////////

// submitWorkflowRequest 是 Argo Server 创建工作流接口需要的请求包装。
type submitWorkflowRequest struct {
	Namespace    string    `json:"namespace"`
	ServerDryRun bool      `json:"serverDryRun"`
	Workflow     *Workflow `json:"workflow"`
}

// SubmitWorkflow 提交新的 Argo Workflow。
// opts.Parameters 会追加到 workflow.spec.arguments.parameters，不会修改调用方传入的 Workflow。
func (wc *WorkflowClient) SubmitWorkflow(ctx context.Context, wf *Workflow, opts *SubmitOptions) (*Workflow, error) {
	if wf == nil {
		return nil, fmt.Errorf("workflow is nil")
	}

	ns := wc.options.Namespace
	if opts != nil && opts.Namespace != "" {
		ns = opts.Namespace
	}

	path := fmt.Sprintf("%s/%s", apiPathPrefix, ns)

	requestWorkflow := wf
	if opts != nil && len(opts.Parameters) > 0 {
		workflowCopy := *wf
		workflowCopy.Spec.Arguments.Parameters = slices.Clone(wf.Spec.Arguments.Parameters)
		for _, parameter := range opts.Parameters {
			name, value, ok := strings.Cut(parameter, "=")
			if !ok || name == "" {
				return nil, fmt.Errorf("invalid workflow parameter %q, expected key=value", parameter)
			}
			workflowCopy.Spec.Arguments.Parameters = append(workflowCopy.Spec.Arguments.Parameters, Parameter{
				Name:  name,
				Value: value,
			})
		}
		requestWorkflow = &workflowCopy
	}

	requestBody := submitWorkflowRequest{
		Namespace: ns,
		Workflow:  requestWorkflow,
	}
	if opts != nil {
		if opts.ServerDryRun {
			requestBody.ServerDryRun = true
		}
	}

	req, err := wc.newRequest(ctx, http.MethodPost, path, requestBody)
	if err != nil {
		return nil, err
	}

	var result Workflow
	err = wc.doRequest(req, &result)
	if err != nil {
		return nil, fmt.Errorf("submit workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("submitted workflow %s in namespace %s", result.Metadata.Name, ns))
	return &result, nil
}

// GetWorkflow 根据名称获取 Workflow。
func (wc *WorkflowClient) GetWorkflow(ctx context.Context, name string, optsNamespace string) (*Workflow, error) {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result Workflow
	err = wc.doRequest(req, &result)
	if err != nil {
		return nil, fmt.Errorf("get workflow error: %w", err)
	}

	return &result, nil
}

// ListWorkflows 列出指定命名空间内的 Workflow。
func (wc *WorkflowClient) ListWorkflows(ctx context.Context, opts *ListOptions) (*WorkflowList, error) {
	ns := wc.options.Namespace
	if opts != nil && opts.Namespace != "" {
		ns = opts.Namespace
	}

	path := fmt.Sprintf("%s/%s", apiPathPrefix, ns)
	if opts != nil {
		params := url.Values{}
		if opts.LabelSelector != "" {
			params.Set("labelSelector", opts.LabelSelector)
		}
		if opts.FieldSelector != "" {
			params.Set("fieldSelector", opts.FieldSelector)
		}
		if opts.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", fmt.Sprintf("%d", opts.Offset))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	req, err := wc.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result WorkflowList
	err = wc.doRequest(req, &result)
	if err != nil {
		return nil, fmt.Errorf("list workflows error: %w", err)
	}

	return &result, nil
}

// DeleteWorkflow 删除指定 Workflow。
func (wc *WorkflowClient) DeleteWorkflow(ctx context.Context, name string, optsNamespace string) error {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	err = wc.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("delete workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("deleted workflow %s in namespace %s", name, ns))
	return nil
}

////////////////////////////////////////////////////////////////////////////////
/// Workflow 生命周期操作
////////////////////////////////////////////////////////////////////////////////

// SuspendWorkflow 挂起正在运行的 Workflow。
func (wc *WorkflowClient) SuspendWorkflow(ctx context.Context, name string, optsNamespace string) error {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/suspend", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return err
	}

	err = wc.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("suspend workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("suspended workflow %s", name))
	return nil
}

// ResumeWorkflow 恢复已挂起的 Workflow。
func (wc *WorkflowClient) ResumeWorkflow(ctx context.Context, name string, optsNamespace string) error {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/resume", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return err
	}

	err = wc.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("resume workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("resumed workflow %s", name))
	return nil
}

// TerminateWorkflow 终止正在运行的 Workflow。
func (wc *WorkflowClient) TerminateWorkflow(ctx context.Context, name string, optsNamespace string) error {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/terminate", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return err
	}

	err = wc.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("terminate workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("terminated workflow %s", name))
	return nil
}

// ResubmitWorkflow 基于已有 Workflow 重新提交一次执行。
func (wc *WorkflowClient) ResubmitWorkflow(ctx context.Context, name string, optsNamespace string) (*Workflow, error) {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/resubmit", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}

	var result Workflow
	err = wc.doRequest(req, &result)
	if err != nil {
		return nil, fmt.Errorf("resubmit workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("resubmitted workflow %s", name))
	return &result, nil
}

// RetryWorkflow 重试失败的 Workflow。
func (wc *WorkflowClient) RetryWorkflow(ctx context.Context, name string, optsNamespace string) (*Workflow, error) {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/retry", apiPathPrefix, ns, name)

	req, err := wc.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}

	var result Workflow
	err = wc.doRequest(req, &result)
	if err != nil {
		return nil, fmt.Errorf("retry workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("retried workflow %s", name))
	return &result, nil
}

// StopWorkflow 停止 Workflow，并可附带停止原因。
func (wc *WorkflowClient) StopWorkflow(ctx context.Context, name string, optsNamespace string, message string) error {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/stop", apiPathPrefix, ns, name)

	body := map[string]string{}
	if message != "" {
		body["message"] = message
	}

	req, err := wc.newRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}

	err = wc.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("stop workflow error: %w", err)
	}

	log.Info(fmt.Sprintf("stopped workflow %s", name))
	return nil
}

////////////////////////////////////////////////////////////////////////////////
/// Workflow 日志
////////////////////////////////////////////////////////////////////////////////

// GetWorkflowLogs 获取工作流或指定 Pod 的日志。
func (wc *WorkflowClient) GetWorkflowLogs(ctx context.Context, name string, optsNamespace string, podName string) (string, error) {
	ns := wc.namespace(optsNamespace)
	path := fmt.Sprintf("%s/%s/%s/log", apiPathPrefix, ns, name)
	if podName != "" {
		params := url.Values{}
		params.Set("podName", podName)
		path += "?" + params.Encode()
	}

	req, err := wc.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	var resp *http.Response
	resp, err = wc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get workflow logs error: %w", err)
	}
	defer resp.Body.Close()

	var data []byte
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read logs error: %w", err)
	}
	// Argo 会把错误详情写入响应体，非 2xx 时必须按调用失败处理。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return string(data), nil
}
