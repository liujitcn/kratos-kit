package argo

import "time"

////////////////////////////////////////////////////////////////////////////////
/// 客户端配置
////////////////////////////////////////////////////////////////////////////////

// ClientOptions 配置 Argo Workflows API 客户端。
type ClientOptions struct {
	// ServerURL 是 Argo Server API 地址，例如 "https://localhost:2746"。
	ServerURL string

	// Namespace 是执行 Workflow 操作的 Kubernetes 命名空间。
	Namespace string

	// Token 是用于认证的 Bearer token，不需要包含 Bearer 前缀。
	Token string

	// InsecureSkipVerify 控制是否跳过 TLS 证书校验，仅建议开发环境使用。
	InsecureSkipVerify bool
}

////////////////////////////////////////////////////////////////////////////////
/// 提交配置
////////////////////////////////////////////////////////////////////////////////

// SubmitOptions 表示提交 Workflow 时的可选参数。
type SubmitOptions struct {
	// Namespace 是目标命名空间，会覆盖客户端默认命名空间。
	Namespace string

	// ServerDryRun 要求服务端只校验请求，不实际创建 Workflow。
	ServerDryRun bool

	// Parameters 是 "key=value" 格式的 Workflow 参数。
	Parameters []string
}

////////////////////////////////////////////////////////////////////////////////
/// 列表查询配置
////////////////////////////////////////////////////////////////////////////////

// ListOptions 表示查询 Workflow 列表时的过滤与分页参数。
type ListOptions struct {
	// Namespace 是目标命名空间，会覆盖客户端默认命名空间。
	Namespace string

	// LabelSelector 按 Kubernetes label 过滤 Workflow。
	LabelSelector string

	// FieldSelector 按 Kubernetes field 过滤 Workflow。
	FieldSelector string

	// Limit 限制返回结果数量。
	Limit int64

	// Offset 是分页偏移量。
	Offset int64
}

////////////////////////////////////////////////////////////////////////////////
/// Workflow 阶段
////////////////////////////////////////////////////////////////////////////////

// Phase 表示 Workflow 当前阶段。
type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
	PhaseError     Phase = "Error"
)

// IsTerminal 判断当前阶段是否为终态。
func (p Phase) IsTerminal() bool {
	return p == PhaseSucceeded || p == PhaseFailed || p == PhaseError
}

////////////////////////////////////////////////////////////////////////////////
/// 默认值
////////////////////////////////////////////////////////////////////////////////

const (
	defaultServerURL = "https://localhost:2746"
	defaultNamespace = "default"
	apiVersion       = "argoproj.io/v1alpha1"
	apiPathPrefix    = "/api/v1/workflows"
)

////////////////////////////////////////////////////////////////////////////////
/// Workflow REST 数据结构
////////////////////////////////////////////////////////////////////////////////

// Workflow 表示 Argo Workflow 资源。
type Workflow struct {
	APIVersion string          `json:"apiVersion,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	Metadata   ObjectMeta      `json:"metadata,omitempty"`
	Spec       WorkflowSpec    `json:"spec,omitempty"`
	Status     *WorkflowStatus `json:"status,omitempty"`
}

// ObjectMeta 表示 Kubernetes 对象元数据。
type ObjectMeta struct {
	Name              string            `json:"name,omitempty"`
	GenerateName      string            `json:"generateName,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	UID               string            `json:"uid,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp *time.Time        `json:"creationTimestamp,omitempty"`
}

// WorkflowSpec 表示 Workflow 规格。
type WorkflowSpec struct {
	Entrypoint         string     `json:"entrypoint,omitempty"`
	Templates          []Template `json:"templates,omitempty"`
	Arguments          Arguments  `json:"arguments,omitempty"`
	ServiceAccountName string     `json:"serviceAccountName,omitempty"`
}

// Template 表示 Workflow 模板。
type Template struct {
	Name      string          `json:"name,omitempty"`
	Container *Container      `json:"container,omitempty"`
	Script    *Script         `json:"script,omitempty"`
	DAG       *DAGTemplate    `json:"dag,omitempty"`
	Steps     []ParallelSteps `json:"steps,omitempty"`
}

// Container 表示容器模板。
type Container struct {
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// Script 表示脚本模板。
type Script struct {
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
	Source  string   `json:"source,omitempty"`
}

// DAGTemplate 表示 DAG 模板。
type DAGTemplate struct {
	Tasks []DAGTask `json:"tasks,omitempty"`
}

// DAGTask 表示 DAG 模板中的任务。
type DAGTask struct {
	Name         string    `json:"name,omitempty"`
	Template     string    `json:"template,omitempty"`
	Arguments    Arguments `json:"arguments,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
}

// ParallelSteps 表示同一阶段内可并行执行的一组 steps。
// Argo 原生 JSON 结构要求 steps 是二维数组：外层表示顺序阶段，内层表示并行步骤。
type ParallelSteps []Step

// Step 表示 steps 模板中的单个步骤。
type Step struct {
	Name      string    `json:"name,omitempty"`
	Template  string    `json:"template,omitempty"`
	Arguments Arguments `json:"arguments,omitempty"`
}

// Arguments 表示 Workflow 或步骤参数。
type Arguments struct {
	Parameters []Parameter `json:"parameters,omitempty"`
}

// Parameter 表示 Workflow 参数。
type Parameter struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// WorkflowStatus 表示 Workflow 状态。
type WorkflowStatus struct {
	Phase      Phase                 `json:"phase,omitempty"`
	StartedAt  *time.Time            `json:"startedAt,omitempty"`
	FinishedAt *time.Time            `json:"finishedAt,omitempty"`
	Message    string                `json:"message,omitempty"`
	Nodes      map[string]NodeStatus `json:"nodes,omitempty"`
}

// NodeStatus 表示 Workflow 中节点的状态。
type NodeStatus struct {
	ID           string     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	TemplateName string     `json:"templateName,omitempty"`
	Phase        Phase      `json:"phase,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	Message      string     `json:"message,omitempty"`
}

// WorkflowList 表示 Workflow 列表。
type WorkflowList struct {
	Items []Workflow `json:"items,omitempty"`
}
