package redisqueue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// ProducerOptions 生产者配置。
type ProducerOptions struct {
	// StreamMaxLength 对应 XADD 的 MAXLEN 配置，用于限制 Stream 总长度，避免消息无限增长占满内存。
	// 这里限制的是 Stream 中的总消息数，而不是“已消费完成”的消息数。
	// 如果消费者全部不可用但生产者仍在持续入队，达到上限后较早的未处理消息也可能被裁剪。
	// 因此生产环境通常建议根据业务峰值将该值设置得更高。
	StreamMaxLength int64
	// ApproximateMaxLength 控制 MAXLEN 是否使用 `~` 近似裁剪，以换取更高的裁剪性能。
	ApproximateMaxLength bool
	// RedisOptions 底层 Redis 连接配置。
	RedisOptions *RedisOptions
}

// Producer Redis Stream 生产者。
type Producer struct {
	options *ProducerOptions
	redis   redis.UniversalClient
}

var defaultProducerOptions = &ProducerOptions{
	StreamMaxLength:      1000,
	ApproximateMaxLength: true,
}

// normalizeProducerOptions 归一化生产者配置并补齐默认值。
func normalizeProducerOptions(options *ProducerOptions) *ProducerOptions {
	if options == nil {
		return &ProducerOptions{
			StreamMaxLength:      defaultProducerOptions.StreamMaxLength,
			ApproximateMaxLength: defaultProducerOptions.ApproximateMaxLength,
		}
	}
	if options.StreamMaxLength <= 0 {
		options.StreamMaxLength = defaultProducerOptions.StreamMaxLength
	}

	return options
}

// NewProducer 使用默认配置创建生产者。
func NewProducer() (*Producer, error) {
	return NewProducerWithOptions(defaultProducerOptions)
}

// NewProducerWithOptions 使用自定义配置创建生产者。
func NewProducerWithOptions(options *ProducerOptions) (*Producer, error) {
	options = normalizeProducerOptions(options)

	redisClient, err := newCheckedRedisClient(options.RedisOptions)
	if err != nil {
		return nil, err
	}

	return &Producer{
		options: options,
		redis:   redisClient,
	}, nil
}

// buildXAddArgs 构造 XADD 参数，并透传长度裁剪相关配置。
func (p *Producer) buildXAddArgs(msg *Message) *redis.XAddArgs {
	args := &redis.XAddArgs{
		ID:     msg.ID,
		Stream: msg.Stream,
		Values: msg.Values,
	}
	args.MaxLen = p.options.StreamMaxLength
	args.Approx = p.options.ApproximateMaxLength
	return args
}

// Enqueue 将消息追加到指定 Stream。
// 除非业务明确需要自定义消息 ID，否则建议交由 Redis 自动生成。
// 若由 Redis 自动生成，生成后的 ID 会回写到 msg.ID。
func (p *Producer) Enqueue(msg *Message) error {
	args := p.buildXAddArgs(msg)
	id, err := p.redis.XAdd(context.TODO(), args).Result()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}
