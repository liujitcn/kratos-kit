package langchaingo

import (
	"context"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
)

// ToRetriever 将 VectorStore 转换为 Retriever。
func ToRetriever(store vectorstores.VectorStore, numDocuments int, opts ...vectorstores.Option) vectorstores.Retriever {
	return vectorstores.ToRetriever(store, numDocuments, opts...)
}

// AddDocuments 向向量存储中添加文档。
func AddDocuments(ctx context.Context, store vectorstores.VectorStore, docs []schema.Document, opts ...vectorstores.Option) ([]string, error) {
	return store.AddDocuments(ctx, docs, opts...)
}

// SimilaritySearch 在向量存储中进行相似度搜索。
func SimilaritySearch(ctx context.Context, store vectorstores.VectorStore, query string, numDocuments int, opts ...vectorstores.Option) ([]schema.Document, error) {
	return store.SimilaritySearch(ctx, query, numDocuments, opts...)
}

// WithNameSpace 设置向量存储命名空间。
func WithNameSpace(nameSpace string) vectorstores.Option {
	return vectorstores.WithNameSpace(nameSpace)
}

// WithScoreThreshold 设置相似度分数阈值。
func WithScoreThreshold(scoreThreshold float32) vectorstores.Option {
	return vectorstores.WithScoreThreshold(scoreThreshold)
}

// WithFilters 设置元数据过滤条件。
func WithFilters(filters any) vectorstores.Option {
	return vectorstores.WithFilters(filters)
}

// WithEmbedder 设置用于向量化的 Embedder。
func WithEmbedder(embedder embeddings.Embedder) vectorstores.Option {
	return vectorstores.WithEmbedder(embedder)
}

// WithDeduplicater 设置添加文档时的去重函数。
func WithDeduplicater(fn func(ctx context.Context, doc schema.Document) bool) vectorstores.Option {
	return vectorstores.WithDeduplicater(fn)
}
