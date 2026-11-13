package eino

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// agenticChunksToMessage 将流 chunk 合并为最终消息，模拟 ADK 图内的合并行为。
func agenticChunksToMessage(t *testing.T, reader *schema.StreamReader[*schema.AgenticMessage]) *schema.AgenticMessage {
	t.Helper()
	defer reader.Close()
	chunks := make([]*schema.AgenticMessage, 0)
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		chunks = append(chunks, chunk)
	}
	// 用 eino 原生合并校验归一化后的流满足其混合块约束。
	msg, err := schema.ConcatAgenticMessages(chunks)
	if err != nil {
		t.Fatalf("ConcatAgenticMessages() error = %v", err)
	}
	return msg
}

// assistantTextChunk 构造携带流式标记的文本增量 chunk。
func assistantTextChunk(text string) *schema.AgenticMessage {
	msg := assistantTextMessage("")
	msg.ContentBlocks = []*schema.ContentBlock{
		schema.NewContentBlockChunk(&schema.AssistantGenText{Text: text}, &schema.StreamingMeta{Index: 0}),
	}
	return msg
}

// assistantTextMessage 构造仅含非流式文本块的助手消息。
func assistantTextMessage(text string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.AssistantGenText{Text: text}),
		},
	}
}

// reasoningChunk 构造单个非流式推理块 chunk。
func reasoningChunk() *schema.AgenticMessage {
	msg := assistantTextMessage("")
	msg.ContentBlocks = []*schema.ContentBlock{
		schema.NewContentBlock(&schema.Reasoning{}),
	}
	return msg
}

// toolCallBlock 构造非流式工具调用块。
func toolCallBlock(name string) *schema.ContentBlock {
	return schema.NewContentBlock(&schema.FunctionToolCall{CallID: "call_1", Name: name})
}

// messageText 取合并消息里的助手文本。
func messageText(msg *schema.AgenticMessage) string {
	var builder strings.Builder
	for _, block := range msg.ContentBlocks {
		if block != nil && block.AssistantGenText != nil {
			builder.WriteString(block.AssistantGenText.Text)
		}
	}
	return builder.String()
}

// toolCallNames 收集合并消息里的工具调用名。
func toolCallNames(msg *schema.AgenticMessage) []string {
	names := make([]string, 0)
	for _, block := range msg.ContentBlocks {
		if block != nil && block.FunctionToolCall != nil {
			names = append(names, block.FunctionToolCall.Name)
		}
	}
	return names
}

// TestNormalizeAgenticStreamMixedBlocks 验证非流式块与流式增量混发时合并仍按时间顺序成功。
func TestNormalizeAgenticStreamMixedBlocks(t *testing.T) {
	reader := NormalizeAgenticStream(schema.StreamReaderFromArray([]*schema.AgenticMessage{reasoningChunk(), assistantTextChunk("你好")}))
	msg := agenticChunksToMessage(t, reader)

	if len(msg.ContentBlocks) != 2 {
		t.Fatalf("ContentBlocks len = %d, want 2", len(msg.ContentBlocks))
	}
	if msg.ContentBlocks[0].Reasoning == nil {
		t.Fatalf("第一个块应为推理块，实际 %#v", msg.ContentBlocks[0])
	}
	if msg.ContentBlocks[1].AssistantGenText == nil || msg.ContentBlocks[1].AssistantGenText.Text != "你好" {
		t.Fatalf("第二个块应为文本块，实际 %#v", msg.ContentBlocks[1])
	}
}

// TestNormalizeAgenticStreamSameIndexChunks 验证同一原 index 的多个增量仍归并到同一顺序位置。
func TestNormalizeAgenticStreamSameIndexChunks(t *testing.T) {
	reader := NormalizeAgenticStream(schema.StreamReaderFromArray([]*schema.AgenticMessage{assistantTextChunk("链路"), assistantTextChunk("正常")}))
	msg := agenticChunksToMessage(t, reader)

	if len(msg.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks len = %d, want 1", len(msg.ContentBlocks))
	}
	if msg.ContentBlocks[0].AssistantGenText.Text != "链路正常" {
		t.Fatalf("合并文本 = %q, want %q", msg.ContentBlocks[0].AssistantGenText.Text, "链路正常")
	}
}

// TestNormalizeAgenticStreamErrorPassthrough 验证上游流错误原样透传给下游。
func TestNormalizeAgenticStreamErrorPassthrough(t *testing.T) {
	sr, sw := schema.Pipe[*schema.AgenticMessage](1)
	sw.Send(nil, errors.New("上游失败"))
	sw.Close()

	reader := NormalizeAgenticStream(sr)
	defer reader.Close()
	if _, err := reader.Recv(); err == nil || err.Error() != "上游失败" {
		t.Fatalf("Recv() error = %v, want 上游失败", err)
	}
}

// TestNormalizeAgenticStreamDropsDuplicateAggregate 验证聚合网关先发完整答案再发逐字增量时，重复的聚合块被丢弃、逐字增量保留。
func TestNormalizeAgenticStreamDropsDuplicateAggregate(t *testing.T) {
	aggregate := assistantTextMessage("链路正常")
	deltas := []*schema.AgenticMessage{assistantTextChunk("链路"), assistantTextChunk("正常")}

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray(append([]*schema.AgenticMessage{aggregate}, deltas...)))
	msg := agenticChunksToMessage(t, reader)

	if text := messageText(msg); text != "链路正常" {
		t.Fatalf("合并文本 = %q, want %q", text, "链路正常")
	}
}

// TestNormalizeAgenticStreamKeepsDistinctAggregate 验证聚合块与后续增量内容不同时两者都保留。
func TestNormalizeAgenticStreamKeepsDistinctAggregate(t *testing.T) {
	aggregate := assistantTextMessage("思考完成")
	deltas := []*schema.AgenticMessage{assistantTextChunk("链路"), assistantTextChunk("正常")}

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray(append([]*schema.AgenticMessage{aggregate}, deltas...)))
	msg := agenticChunksToMessage(t, reader)

	if text := messageText(msg); text != "思考完成链路正常" {
		t.Fatalf("合并文本 = %q, want %q", text, "思考完成链路正常")
	}
}

// TestNormalizeAgenticStreamFlushAggregateOnly 验证只有聚合块没有增量的流在结束时原样转发。
func TestNormalizeAgenticStreamFlushAggregateOnly(t *testing.T) {
	aggregate := assistantTextMessage("仅聚合内容")

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray([]*schema.AgenticMessage{aggregate}))
	msg := agenticChunksToMessage(t, reader)

	if text := messageText(msg); text != "仅聚合内容" {
		t.Fatalf("合并文本 = %q, want %q", text, "仅聚合内容")
	}
}

// TestNormalizeAgenticStreamKeepsToolCallFromAggregate 验证聚合块中的工具调用不被去重丢弃。
func TestNormalizeAgenticStreamKeepsToolCallFromAggregate(t *testing.T) {
	aggregate := assistantTextMessage("")
	aggregate.ContentBlocks = []*schema.ContentBlock{toolCallBlock("html_to_pdf")}
	deltas := []*schema.AgenticMessage{assistantTextChunk("报告内容")}

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray(append([]*schema.AgenticMessage{aggregate}, deltas...)))
	msg := agenticChunksToMessage(t, reader)

	names := toolCallNames(msg)
	if len(names) != 1 || names[0] != "html_to_pdf" {
		t.Fatalf("工具调用块丢失或重复: %v", names)
	}
}

// TestNormalizeAgenticStreamKeepsToolCallMixedAggregate 验证文本与工具调用混在同一聚合块时各自独立处理。
func TestNormalizeAgenticStreamKeepsToolCallMixedAggregate(t *testing.T) {
	aggregate := assistantTextMessage("正在生成报告")
	aggregate.ContentBlocks = append(aggregate.ContentBlocks, toolCallBlock("merge_pdfs"))
	deltas := []*schema.AgenticMessage{assistantTextChunk("正在生成报告")}

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray([]*schema.AgenticMessage{aggregate, deltas[0]}))
	msg := agenticChunksToMessage(t, reader)

	if text := messageText(msg); strings.Count(text, "正在生成报告") != 1 {
		t.Fatalf("文本应去重为一遍, got %q", text)
	}
	names := toolCallNames(msg)
	if len(names) != 1 || names[0] != "merge_pdfs" {
		t.Fatalf("混合聚合块中的工具调用丢失: %v", names)
	}
}

// TestNormalizeAgenticStreamSeparatesMixedIndexGroups 验证文本与工具调用共享同一原始 index 时仍按类型分组各自合并。
func TestNormalizeAgenticStreamSeparatesMixedIndexGroups(t *testing.T) {
	chunks := make([]*schema.AgenticMessage, 0)
	// 文本增量与工具参数增量交错到达，原始 index 均为 0（聚合网关同 index 混组形态）。
	texts := []string{"Go", " 是", " 一门", " 语言"}
	args := []string{"{\"f", "ile\":", "\"a.pdf\"}"}
	for _, text := range texts {
		msg := assistantTextMessage("")
		msg.ContentBlocks = []*schema.ContentBlock{
			schema.NewContentBlockChunk(&schema.AssistantGenText{Text: text}, &schema.StreamingMeta{Index: 0}),
		}
		chunks = append(chunks, msg)
	}
	for _, argsText := range args {
		msg := assistantTextMessage("")
		msg.ContentBlocks = []*schema.ContentBlock{
			schema.NewContentBlockChunk(&schema.FunctionToolCall{CallID: "call_1", Name: "html_to_pdf", Arguments: argsText}, &schema.StreamingMeta{Index: 0}),
		}
		chunks = append(chunks, msg)
	}

	reader := NormalizeAgenticStream(schema.StreamReaderFromArray(chunks))
	msg := agenticChunksToMessage(t, reader)

	if text := messageText(msg); text != "Go 是 一门 语言" {
		t.Fatalf("合并文本 = %q, want %q", text, "Go 是 一门 语言")
	}
	names := toolCallNames(msg)
	if len(names) != 1 || names[0] != "html_to_pdf" {
		t.Fatalf("同 index 的工具增量应合并为一次调用: %v", names)
	}
}

// TestNewNormalizedAgenticModelWrapsStream 验证包装模型的 Stream 出口已做归一化。
func TestNewNormalizedAgenticModelWrapsStream(t *testing.T) {
	inner := &streamProbeModel{}
	wrapped := NewNormalizedAgenticModel(inner)

	reader, err := wrapped.Stream(context.Background(), []*schema.AgenticMessage{{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.UserInputText{Text: "测试"}),
		},
	}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	msg := agenticChunksToMessage(t, reader)
	if messageText(msg) != "混合回复" {
		t.Fatalf("文本 = %q, want %q", messageText(msg), "混合回复")
	}
	if !inner.streamCalled {
		t.Fatal("内部模型 Stream 未被调用")
	}
}

type streamProbeModel struct {
	streamCalled bool
}

func (m *streamProbeModel) Generate(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.AgenticMessage, error) {
	return assistantTextMessage("完整回复"), nil
}

func (m *streamProbeModel) Stream(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	m.streamCalled = true
	return schema.StreamReaderFromArray([]*schema.AgenticMessage{reasoningChunk(), assistantTextChunk("混合回复")}), nil
}
