package eino

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Lambda 透传 Eino Lambda 类型。
type Lambda = compose.Lambda

// LambdaOption 透传 Eino Lambda 选项。
type LambdaOption = compose.LambdaOpt

// InvokableLambda 创建一个仅支持 Invoke 模式的 Lambda 节点。
func InvokableLambda[I, O any](fn func(ctx context.Context, input I) (O, error), opts ...compose.LambdaOpt) *compose.Lambda {
	return compose.InvokableLambda(fn, opts...)
}

// StreamableLambda 创建一个支持 Stream 模式的 Lambda 节点。
func StreamableLambda[I, O any](fn func(ctx context.Context, input I) (*schema.StreamReader[O], error), opts ...compose.LambdaOpt) *compose.Lambda {
	return compose.StreamableLambda(fn, opts...)
}

// CollectableLambda 创建一个支持 Collect 模式的 Lambda 节点。
func CollectableLambda[I, O any](fn func(ctx context.Context, input *schema.StreamReader[I]) (O, error), opts ...compose.LambdaOpt) *compose.Lambda {
	return compose.CollectableLambda(fn, opts...)
}

// TransformableLambda 创建一个支持 Transform 模式的 Lambda 节点。
func TransformableLambda[I, O any](fn func(ctx context.Context, input *schema.StreamReader[I]) (*schema.StreamReader[O], error), opts ...compose.LambdaOpt) *compose.Lambda {
	return compose.TransformableLambda(fn, opts...)
}

// AnyLambda 创建一个支持多种模式的 Lambda 节点。
func AnyLambda[I, O, TOption any](
	invoke compose.Invoke[I, O, TOption],
	stream compose.Stream[I, O, TOption],
	collect compose.Collect[I, O, TOption],
	transform compose.Transform[I, O, TOption],
	opts ...compose.LambdaOpt,
) (*compose.Lambda, error) {
	return compose.AnyLambda(invoke, stream, collect, transform, opts...)
}

// ToList 创建一个将单个输入转换为切片的 Lambda 节点。
func ToList[I any](opts ...compose.LambdaOpt) *compose.Lambda {
	return compose.ToList[I](opts...)
}

// Parallel 透传 Eino Parallel 类型。
type Parallel = compose.Parallel

// NewParallel 创建一个并行结构，用于在 Chain 中同时执行多个节点。
func NewParallel() *compose.Parallel {
	return compose.NewParallel()
}

// ChainBranch 透传 Eino ChainBranch 类型。
type ChainBranch = compose.ChainBranch

// GraphBranchCondition 透传 Eino GraphBranchCondition 类型。
type GraphBranchCondition[T any] = compose.GraphBranchCondition[T]

// NewChainBranch 创建一个条件分支。
func NewChainBranch[T any](cond compose.GraphBranchCondition[T]) *compose.ChainBranch {
	return compose.NewChainBranch(cond)
}

// NewWorkflow 创建一个支持依赖管理和复杂数据流转的工作流。
func NewWorkflow[I, O any](opts ...compose.NewGraphOption) *compose.Workflow[I, O] {
	return compose.NewWorkflow[I, O](opts...)
}

// WorkflowNode 透传 Eino WorkflowNode 类型。
type WorkflowNode = compose.WorkflowNode

// Runnable 表示编译后的可执行 Chain、Graph 或 Workflow。
type Runnable[I, O any] = compose.Runnable[I, O]

// WithGenLocalState 注册一个函数来生成每次运行时的本地状态。
func WithGenLocalState[S any](gls func(ctx context.Context) S) compose.NewGraphOption {
	return compose.WithGenLocalState(gls)
}

// GraphAddNodeOpt 透传 Eino GraphAddNodeOpt 类型。
type GraphAddNodeOpt = compose.GraphAddNodeOpt

// GraphCompileOption 透传 Eino GraphCompileOption 类型。
type GraphCompileOption = compose.GraphCompileOption
