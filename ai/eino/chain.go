package eino

import (
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
)

// NewChain 创建一个泛型 Chain，用于将多个组件按顺序串联。
func NewChain[I, O any](opts ...compose.NewGraphOption) *compose.Chain[I, O] {
	return compose.NewChain[I, O](opts...)
}

// NewGraph 创建一个泛型 Graph，用于构建更复杂的 DAG 工作流。
func NewGraph[I, O any](opts ...compose.NewGraphOption) *compose.Graph[I, O] {
	return compose.NewGraph[I, O](opts...)
}

const (
	// START 标识 Graph 起始节点。
	START = compose.START
	// END 标识 Graph 结束节点。
	END = compose.END
)

// AppendChatModel 向 Chain 追加 ChatModel 节点。
func AppendChatModel[I, O any](chain *compose.Chain[I, O], node model.BaseChatModel, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendChatModel(node, opts...)
}

// AppendAgenticModel 向 Chain 追加 AgenticModel 节点。
func AppendAgenticModel[I, O any](chain *compose.Chain[I, O], node model.AgenticModel, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendAgenticModel(node, opts...)
}

// AppendChatTemplate 向 Chain 追加 ChatTemplate 节点。
func AppendChatTemplate[I, O any](chain *compose.Chain[I, O], node prompt.ChatTemplate, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendChatTemplate(node, opts...)
}

// AppendAgenticChatTemplate 向 Chain 追加 AgenticChatTemplate 节点。
func AppendAgenticChatTemplate[I, O any](chain *compose.Chain[I, O], node prompt.AgenticChatTemplate, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendAgenticChatTemplate(node, opts...)
}

// AppendToolsNode 向 Chain 追加 ToolsNode 节点。
func AppendToolsNode[I, O any](chain *compose.Chain[I, O], node *compose.ToolsNode, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendToolsNode(node, opts...)
}

// AppendAgenticToolsNode 向 Chain 追加 AgenticToolsNode 节点。
func AppendAgenticToolsNode[I, O any](chain *compose.Chain[I, O], node *compose.AgenticToolsNode, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendAgenticToolsNode(node, opts...)
}

// AppendEmbedding 向 Chain 追加 Embedding 节点。
func AppendEmbedding[I, O any](chain *compose.Chain[I, O], node embedding.Embedder, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendEmbedding(node, opts...)
}

// AppendRetriever 向 Chain 追加 Retriever 节点。
func AppendRetriever[I, O any](chain *compose.Chain[I, O], node retriever.Retriever, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendRetriever(node, opts...)
}

// AppendLoader 向 Chain 追加文档加载器节点。
func AppendLoader[I, O any](chain *compose.Chain[I, O], node document.Loader, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendLoader(node, opts...)
}

// AppendDocumentTransformer 向 Chain 追加文档转换器节点。
func AppendDocumentTransformer[I, O any](chain *compose.Chain[I, O], node document.Transformer, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendDocumentTransformer(node, opts...)
}

// AppendIndexer 向 Chain 追加 Indexer 节点。
func AppendIndexer[I, O any](chain *compose.Chain[I, O], node indexer.Indexer, opts ...compose.GraphAddNodeOpt) *compose.Chain[I, O] {
	return chain.AppendIndexer(node, opts...)
}

// CompileChain 编译 Chain 为可执行的 Runnable。
func CompileChain[I, O any](ctx context.Context, chain *compose.Chain[I, O], opts ...compose.GraphCompileOption) (compose.Runnable[I, O], error) {
	return chain.Compile(ctx, opts...)
}
