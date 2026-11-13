package eino

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// normalizedAgenticModel 将模型流式输出归一化后再交给调用方的模型包装。
type normalizedAgenticModel struct {
	inner model.AgenticModel
}

// NewNormalizedAgenticModel 包装模型，使其流式输出块序列始终满足 eino 合并约束且不含重复内容。
func NewNormalizedAgenticModel(inner model.AgenticModel) model.AgenticModel {
	return &normalizedAgenticModel{inner: inner}
}

// Generate 非流式调用直接透传；完整消息没有合并顺序与重复问题。
func (m *normalizedAgenticModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	return m.inner.Generate(ctx, input, opts...)
}

// Stream 对模型流式输出做块归一化与重复聚合块剔除。部分聚合网关的流式响应会先下发
// 完整答案的聚合块，再下发同一答案的逐字增量；直接消费会因混合块序列触发 eino 合并
// 错误（found streaming block after non-streaming blocks），并且前端与落库各出现两份
// 相同内容。这里统一为全流式块并重排 index，在能确认重复时丢弃聚合块、保留逐字增量。
func (m *normalizedAgenticModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	reader, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return NormalizeAgenticStream(reader), nil
}

// NormalizeAgenticStream 逐 chunk 透传模型流，完成块 index 归一化与聚合重复块剔除。
func NormalizeAgenticStream(reader *schema.StreamReader[*schema.AgenticMessage]) *schema.StreamReader[*schema.AgenticMessage] {
	sr, sw := schema.Pipe[*schema.AgenticMessage](1)
	normalizer := &streamNormalizer{indexMap: map[string]int{}}
	go func() {
		defer reader.Close()
		defer sw.Close()
		for {
			chunk, err := reader.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					// 流正常结束时兜底转发仍缓冲的聚合块：只有聚合没有增量的网关需要原样下发。
					for _, forwarded := range normalizer.flush() {
						sw.Send(forwarded, nil)
					}
				} else {
					// 错误路径原样透传，不补发缓冲的聚合块，避免错误回复之上叠加半截内容。
					sw.Send(nil, err)
				}
				return
			}
			for _, forwarded := range normalizer.process(chunk) {
				sw.Send(forwarded, nil)
			}
		}
	}()
	return sr
}

// streamNormalizer 维护单个流的块 index 重映射与聚合块去重状态。
// 同一个流严格顺序消费，不存在并发访问。
type streamNormalizer struct {
	indexMap map[string]int
	// nextIndex 是下一个可分配的归一化 index，保证块排序与到达时间顺序一致。
	nextIndex int
	// buffered 是待定的聚合 chunk：在确认后续增量是否与其重复前暂不转发。
	buffered []*schema.AgenticMessage
	// aggText 是缓冲聚合块的累计文本。
	aggText strings.Builder
	// deltaText 是聚合块之后增量组的累计文本，用于判断重复。
	deltaText strings.Builder
	// settled 表示重复判断已经完成（聚合块已决定转发或丢弃）。
	settled bool
	// aggDropped 表示聚合块已确认为重复并被丢弃。
	aggDropped bool
}

// process 处理单个 chunk，返回应当转发给下游的 chunk 列表（可能为空、也可能先补发缓冲的聚合块）。
func (n *streamNormalizer) process(chunk *schema.AgenticMessage) []*schema.AgenticMessage {
	if chunk == nil {
		return nil
	}
	out := make([]*schema.AgenticMessage, 0, 2)
	// 工具调用等服务端块不参与聚合去重：它们携带工具名等结构化信息，增量流里只有参数
	// 增量，整块丢弃聚合块会把工具调用本身丢掉，导致下游拿到无名工具调用。
	textChunk, otherChunk := splitGenerativeBlocks(chunk)
	if otherChunk != nil {
		n.remap(otherChunk)
		out = append(out, otherChunk)
	}
	if textChunk == nil {
		return out
	}
	chunk = textChunk
	if n.settled {
		n.remap(chunk)
		return append(out, chunk)
	}
	if hasNonStreamingBlock(chunk) {
		// 聚合阶段：累计缓冲聚合文本块，等待后续增量以判断重复。
		n.aggText.WriteString(agenticText(chunk))
		n.buffer(chunk)
		return out
	}
	if agenticText(chunk) == "" {
		// 无文本 chunk 不参与重复判断，直接转发。
		n.remap(chunk)
		return append(out, chunk)
	}
	// 增量阶段：增量是聚合文本的前缀则确认重复，丢弃聚合块保留逐字增量；
	// 增量内容偏离聚合文本说明两者不同，先把聚合块转发出去再透传增量。
	n.deltaText.WriteString(agenticText(chunk))
	if strings.HasPrefix(n.aggText.String(), n.deltaText.String()) {
		n.settled = true
		n.aggDropped = true
		n.buffered = nil
	} else {
		n.settled = true
	}
	if !n.aggDropped {
		for _, buffered := range n.buffered {
			n.remap(buffered)
			out = append(out, buffered)
		}
	}
	n.buffered = nil
	n.remap(chunk)
	return append(out, chunk)
}

// flush 在流结束时兜底处理仍处缓冲态的聚合块：只有聚合没有增量的网关需要原样转发。
func (n *streamNormalizer) flush() []*schema.AgenticMessage {
	if n.settled || len(n.buffered) == 0 {
		return nil
	}
	out := make([]*schema.AgenticMessage, 0, len(n.buffered))
	for _, buffered := range n.buffered {
		n.remap(buffered)
		out = append(out, buffered)
	}
	n.buffered = nil
	n.settled = true
	return out
}

// buffer 缓冲待定的聚合 chunk。
func (n *streamNormalizer) buffer(chunk *schema.AgenticMessage) {
	n.buffered = append(n.buffered, chunk)
}

// remap 将 chunk 内的块标记统一为按到达顺序递增的流式 index。
// 映射 key 携带块类型：聚合网关会把文本与工具调用的增量塞进同一个原始 index，
// 而 eino 合并时按 index 分组并按首块类型选择合并策略，混组会导致文本丢失、
// 工具调用各自独立成块（无名块触发 tool not found）。
func (n *streamNormalizer) remap(chunk *schema.AgenticMessage) {
	if chunk == nil {
		return
	}
	for _, block := range chunk.ContentBlocks {
		if block == nil {
			continue
		}
		if block.StreamingMeta != nil {
			key := remapKey(block)
			mapped, ok := n.indexMap[key]
			if !ok {
				mapped = n.nextIndex
				n.nextIndex++
				n.indexMap[key] = mapped
			}
			block.StreamingMeta.Index = mapped
			continue
		}
		// 非流式块视为一次性到达的完整块，同样占用一个顺序位置参与后续合并排序。
		block.StreamingMeta = &schema.StreamingMeta{Index: n.nextIndex}
		n.nextIndex++
	}
}

// remapKey 返回块的归一化分组 key：原始 index + 块类型。
func remapKey(block *schema.ContentBlock) string {
	return fmt.Sprintf("%d:%v", block.StreamingMeta.Index, block.Type)
}

// splitGenerativeBlocks 把 chunk 的内容块拆成生成类块（文本/思考）与结构化块（工具调用等）
// 两组，仅生成类块参与聚合去重。返回值在整组为空时为 nil。
func splitGenerativeBlocks(chunk *schema.AgenticMessage) (textChunk *schema.AgenticMessage, otherChunk *schema.AgenticMessage) {
	blocks := chunk.ContentBlocks
	textBlocks := make([]*schema.ContentBlock, 0, len(blocks))
	otherBlocks := make([]*schema.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if isGenerativeBlock(block) {
			textBlocks = append(textBlocks, block)
			continue
		}
		otherBlocks = append(otherBlocks, block)
	}
	if len(textBlocks) == len(blocks) {
		return chunk, nil
	}
	if len(otherBlocks) == len(blocks) {
		return nil, chunk
	}
	text := *chunk
	text.ContentBlocks = textBlocks
	other := *chunk
	other.ContentBlocks = otherBlocks
	return &text, &other
}

// isGenerativeBlock 判断块是否为生成类内容（文本或思考），这些块可能被聚合网关重复下发。
func isGenerativeBlock(block *schema.ContentBlock) bool {
	return block.AssistantGenText != nil || block.Reasoning != nil
}

// hasNonStreamingBlock 判断 chunk 是否携带非流式块。
func hasNonStreamingBlock(chunk *schema.AgenticMessage) bool {
	for _, block := range chunk.ContentBlocks {
		if block != nil && block.StreamingMeta == nil {
			return true
		}
	}
	return false
}

// agenticText 取 chunk 内助手生成文本，聚合与增量内容重复判断均基于该文本。
func agenticText(chunk *schema.AgenticMessage) string {
	var builder strings.Builder
	for _, block := range chunk.ContentBlocks {
		if block != nil && block.AssistantGenText != nil {
			builder.WriteString(block.AssistantGenText.Text)
		}
	}
	return builder.String()
}
