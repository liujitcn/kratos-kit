package eino

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

// NewToolNode 创建一个工具执行节点。
func NewToolNode(ctx context.Context, config *compose.ToolsNodeConfig) (*compose.ToolsNode, error) {
	return compose.NewToolNode(ctx, config)
}

// NewAgenticToolNode 创建一个 Agentic 工具执行节点。
func NewAgenticToolNode(ctx context.Context, config *compose.ToolsNodeConfig) (*compose.AgenticToolsNode, error) {
	return compose.NewAgenticToolsNode(ctx, config)
}

// ToolsNodeConfig 透传 Eino ToolsNodeConfig 类型。
type ToolsNodeConfig = compose.ToolsNodeConfig

// AgenticToolsNode 透传 Eino AgenticToolsNode 类型。
type AgenticToolsNode = compose.AgenticToolsNode

// ToolsNodeOption 透传 Eino ToolsNodeOption 类型。
type ToolsNodeOption = compose.ToolsNodeOption

// WithToolOption 设置工具调用的额外选项。
func WithToolOption(opts ...tool.Option) compose.ToolsNodeOption {
	return compose.WithToolOption(opts...)
}

// WithToolList 设置可调用的工具列表。
func WithToolList(tools ...tool.BaseTool) compose.ToolsNodeOption {
	return compose.WithToolList(tools...)
}

// NewToolInfo 创建一个工具描述信息。
func NewToolInfo(name, desc string, params *schema.ParamsOneOf) *schema.ToolInfo {
	return &schema.ToolInfo{
		Name:        name,
		Desc:        desc,
		ParamsOneOf: params,
	}
}

// NewParamsOneOfByParams 通过参数描述映射创建工具参数定义。
func NewParamsOneOfByParams(params map[string]*schema.ParameterInfo) *schema.ParamsOneOf {
	return schema.NewParamsOneOfByParams(params)
}

// NewParamsOneOfByJSONSchema 通过 JSON Schema 创建工具参数定义。
func NewParamsOneOfByJSONSchema(s *jsonschema.Schema) *schema.ParamsOneOf {
	return schema.NewParamsOneOfByJSONSchema(s)
}

// NewParameterInfo 创建一个参数描述。
func NewParameterInfo(typ schema.DataType, desc string, required bool, opts ...ParameterInfoOption) *schema.ParameterInfo {
	pi := &schema.ParameterInfo{
		Type:     typ,
		Desc:     desc,
		Required: required,
	}
	for _, opt := range opts {
		opt(pi)
	}
	return pi
}

// ParameterInfoOption 用于配置 ParameterInfo 的额外字段。
type ParameterInfoOption func(*schema.ParameterInfo)

// WithEnum 设置参数的枚举值。
func WithEnum(enum []string) ParameterInfoOption {
	return func(pi *schema.ParameterInfo) {
		pi.Enum = enum
	}
}

// WithSubParams 设置对象参数的子参数。
func WithSubParams(subParams map[string]*schema.ParameterInfo) ParameterInfoOption {
	return func(pi *schema.ParameterInfo) {
		pi.SubParams = subParams
	}
}

// WithElemInfo 设置数组参数的元素类型。
func WithElemInfo(elem *schema.ParameterInfo) ParameterInfoOption {
	return func(pi *schema.ParameterInfo) {
		pi.ElemInfo = elem
	}
}
