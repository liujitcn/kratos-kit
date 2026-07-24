package langchaingo

import (
	"context"

	"github.com/tmc/langchaingo/embeddings"
)

// NewEmbedder 基于 EmbedderClient 创建文本向量化器。
func NewEmbedder(client embeddings.EmbedderClient, opts ...embeddings.Option) (*embeddings.EmbedderImpl, error) {
	return embeddings.NewEmbedder(client, opts...)
}

// EmbedQuery 基于 EmbedderClient 对单个文本进行向量化。
func EmbedQuery(ctx context.Context, client embeddings.EmbedderClient, text string) ([]float32, error) {
	embedder, err := embeddings.NewEmbedder(client)
	if err != nil {
		return nil, err
	}
	return embedder.EmbedQuery(ctx, text)
}

// EmbedDocuments 基于 EmbedderClient 对多个文本进行向量化。
func EmbedDocuments(ctx context.Context, client embeddings.EmbedderClient, texts []string) ([][]float32, error) {
	embedder, err := embeddings.NewEmbedder(client)
	if err != nil {
		return nil, err
	}
	return embedder.EmbedDocuments(ctx, texts)
}

// WithStripNewLines 设置是否去除文本中的换行符。
func WithStripNewLines(stripNewLines bool) embeddings.Option {
	return embeddings.WithStripNewLines(stripNewLines)
}

// WithBatchSize 设置批量处理的文档数量。
func WithBatchSize(batchSize int) embeddings.Option {
	return embeddings.WithBatchSize(batchSize)
}
