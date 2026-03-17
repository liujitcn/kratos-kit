package redis

import (
	"fmt"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/queue/redisqueue"
	"github.com/liujitcn/kratos-kit/utils"
)

// Redis cache implement
type Redis struct {
	consumer *redisqueue.Consumer
	producer *redisqueue.Producer
}

// NewRedis 创建 Redis 队列实现。
func NewRedis(redisCfg *conf.Data_Redis, queueCfg *conf.Data_Queue) (*Redis, error) {
	if redisCfg == nil {
		return nil, fmt.Errorf("queue redis config is nil")
	}
	redisOptions, err := utils.GetUniversalOptions(redisCfg)
	if err != nil {
		return nil, fmt.Errorf("build redis options failed: %w", err)
	}

	consumerOptions := &redisqueue.ConsumerOptions{
		RedisOptions: redisOptions,
	}

	producerOptions := &redisqueue.ProducerOptions{
		RedisOptions: redisOptions,
	}
	if queueCfg != nil {
		queueRedisConf := queueCfg.Redis
		if queueRedisConf != nil {
			consumerConf := queueRedisConf.Consumer
			if consumerConf != nil {
				consumerOptions.VisibilityTimeout = consumerConf.VisibilityTimeout.AsDuration()
				consumerOptions.BlockingTimeout = consumerConf.BlockingTimeout.AsDuration()
				consumerOptions.ReclaimInterval = consumerConf.ReclaimInterval.AsDuration()
				consumerOptions.BufferSize = int(consumerConf.BufferSize)
				consumerOptions.Concurrency = int(consumerConf.Concurrency)
			}
			producerConf := queueRedisConf.Producer
			if producerConf != nil {
				producerOptions.StreamMaxLength = producerConf.StreamMaxLength
				producerOptions.ApproximateMaxLength = producerConf.ApproximateMaxLength
			}
		}
	}

	var consumer *redisqueue.Consumer
	var producer *redisqueue.Producer
	consumer, err = redisqueue.NewConsumerWithOptions(consumerOptions)
	if err != nil {
		return nil, fmt.Errorf("create redis consumer failed: %w", err)
	}
	producer, err = redisqueue.NewProducerWithOptions(producerOptions)
	if err != nil {
		return nil, fmt.Errorf("create redis producer failed: %w", err)
	}
	return &Redis{
		consumer: consumer,
		producer: producer,
	}, nil
}

// Append 追加消息到 Redis 队列。
func (s *Redis) Append(stream string, message data.Message) error {
	err := s.producer.Enqueue(&redisqueue.Message{
		ID:     message.ID,
		Stream: stream,
		Values: message.Values,
	})
	return err
}

// Register 注册 Redis 队列消费处理函数。
func (s *Redis) Register(stream string, fn data.ConsumerFunc) {
	s.consumer.Register(stream, func(message *redisqueue.Message) error {
		return fn(data.Message{
			ID:     message.ID,
			Values: message.Values,
		})
	})
}

// Run 启动 Redis 队列消费。
func (s *Redis) Run() {
	if s.consumer != nil {
		s.consumer.Run()
	}
}

// Shutdown 关闭 Redis 队列消费。
func (s *Redis) Shutdown() {
	if s.consumer != nil {
		s.consumer.Shutdown()
	}
}
