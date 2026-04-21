package redis

import (
	"fmt"
	"sync"
	"time"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/queue/redisqueue"
	"github.com/liujitcn/kratos-kit/utils"
	"google.golang.org/protobuf/types/known/durationpb"
)

type queueConsumer interface {
	Register(stream string, fn redisqueue.ConsumerFunc)
	Run()
	Shutdown()
}

type queueProducer interface {
	Enqueue(msg *redisqueue.Message) error
}

// Redis Redis Stream 队列实现。
type Redis struct {
	consumer queueConsumer
	producer queueProducer

	mux     sync.Mutex
	running bool
	wait    sync.WaitGroup
}

// durationValue 安全读取可选时长配置，缺省时返回零值以便后续统一走默认值补齐逻辑。
func durationValue(value *durationpb.Duration) time.Duration {
	if value == nil {
		return 0
	}

	return value.AsDuration()
}

// buildConsumerOptions 构造 Redis 队列消费者配置。
func buildConsumerOptions(redisOptions *redisqueue.RedisOptions, queueCfg *conf.Data_Queue) *redisqueue.ConsumerOptions {
	consumerOptions := &redisqueue.ConsumerOptions{
		RedisOptions: redisOptions,
	}

	if queueCfg == nil || queueCfg.Redis == nil || queueCfg.Redis.Consumer == nil {
		return consumerOptions
	}

	consumerConf := queueCfg.Redis.Consumer
	consumerOptions.VisibilityTimeout = durationValue(consumerConf.VisibilityTimeout)
	consumerOptions.BlockingTimeout = durationValue(consumerConf.BlockingTimeout)
	consumerOptions.ReclaimInterval = durationValue(consumerConf.ReclaimInterval)
	consumerOptions.BufferSize = int(consumerConf.BufferSize)
	consumerOptions.Concurrency = int(consumerConf.Concurrency)

	return consumerOptions
}

// buildProducerOptions 构造 Redis 队列生产者配置。
func buildProducerOptions(redisOptions *redisqueue.RedisOptions, queueCfg *conf.Data_Queue) *redisqueue.ProducerOptions {
	producerOptions := &redisqueue.ProducerOptions{
		RedisOptions:         redisOptions,
		ApproximateMaxLength: true,
	}

	if queueCfg == nil || queueCfg.Redis == nil || queueCfg.Redis.Producer == nil {
		return producerOptions
	}

	producerConf := queueCfg.Redis.Producer
	producerOptions.StreamMaxLength = producerConf.StreamMaxLength
	producerOptions.ApproximateMaxLength = producerConf.ApproximateMaxLength

	return producerOptions
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

	consumerOptions := buildConsumerOptions(redisOptions, queueCfg)
	producerOptions := buildProducerOptions(redisOptions, queueCfg)

	consumer, err := redisqueue.NewConsumerWithOptions(consumerOptions)
	if err != nil {
		return nil, fmt.Errorf("create redis consumer failed: %w", err)
	}

	producer, err := redisqueue.NewProducerWithOptions(producerOptions)
	if err != nil {
		return nil, fmt.Errorf("create redis producer failed: %w", err)
	}

	return &Redis{
		consumer: consumer,
		producer: producer,
	}, nil
}

// start 在后台启动 Redis 队列消费循环。
func (s *Redis) start() {
	if s.consumer == nil {
		return
	}

	s.mux.Lock()
	if s.running {
		s.mux.Unlock()
		return
	}
	s.running = true
	s.wait.Add(1)
	s.mux.Unlock()

	go func() {
		defer s.wait.Done()
		defer func() {
			s.mux.Lock()
			s.running = false
			s.mux.Unlock()
		}()
		s.consumer.Run()
	}()
}

// Append 追加消息到 Redis 队列。
func (s *Redis) Append(stream string, message data.Message) error {
	return s.producer.Enqueue(&redisqueue.Message{
		ID:     message.ID,
		Stream: stream,
		Values: message.Values,
	})
}

// Register 注册 Redis 队列消费处理函数，并在首次注册后自动启动消费循环。
func (s *Redis) Register(stream string, fn data.ConsumerFunc) {
	s.consumer.Register(stream, func(message *redisqueue.Message) error {
		return fn(data.Message{
			ID:     message.ID,
			Values: message.Values,
		})
	})
	s.start()
}

// Run 启动 Redis 队列消费，并阻塞等待其结束。
func (s *Redis) Run() {
	s.start()
	s.wait.Wait()
}

// Shutdown 关闭 Redis 队列消费。
func (s *Redis) Shutdown() {
	if s.consumer != nil {
		s.consumer.Shutdown()
	}
}
