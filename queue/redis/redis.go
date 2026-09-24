package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/queue/redisqueue"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/v9"
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
	delayed  redis.UniversalClient

	mux           sync.Mutex
	running       bool
	wait          sync.WaitGroup
	delayedCancel context.CancelFunc
}

const (
	delayedScheduleKey = "kratos:queue:delayed:schedule"
	delayedPayloadKey  = "kratos:queue:delayed:payload"
	delayedLockPrefix  = "kratos:queue:delayed:lock:"
	delayedBatchSize   = 100
	delayedPollPeriod  = 500 * time.Millisecond
	delayedLockTTL     = 30 * time.Second
)

type delayedMessage struct {
	Stream  string       `json:"stream"`
	Message data.Message `json:"message"`
}

// durationValue 安全读取可选时长配置，缺省时返回零值以便后续统一走默认值补齐逻辑。
func durationValue(value *durationpb.Duration) time.Duration {
	if value == nil {
		return 0
	}

	return value.AsDuration()
}

// buildConsumerOptions 构造 Redis 队列消费者配置。
func buildConsumerOptions(redisOptions *redisqueue.RedisOptions, queueCfg *configv1.Data_Queue) *redisqueue.ConsumerOptions {
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
func buildProducerOptions(redisOptions *redisqueue.RedisOptions, queueCfg *configv1.Data_Queue) *redisqueue.ProducerOptions {
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
func NewRedis(redisCfg *configv1.Data_Redis, queueCfg *configv1.Data_Queue) (*Redis, error) {
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

	delayedClient := redis.NewUniversalClient(redisOptions)
	if err = delayedClient.Ping(context.Background()).Err(); err != nil {
		_ = delayedClient.Close()
		return nil, fmt.Errorf("create delayed redis client failed: %w", err)
	}

	return &Redis{
		consumer: consumer,
		producer: producer,
		delayed:  delayedClient,
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
	delayedCtx, delayedCancel := context.WithCancel(context.Background())
	s.delayedCancel = delayedCancel
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
	go func() {
		defer s.wait.Done()
		s.runDelayed(delayedCtx)
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

// Schedule 按指定时间保存 Redis 延迟消息；相同流和消息编号会覆盖原计划。
func (s *Redis) Schedule(stream string, message data.Message, executeAt time.Time) error {
	if stream == "" {
		return errors.New("queue stream is empty")
	}
	if message.ID == "" {
		return errors.New("delayed message id is empty")
	}
	payload, err := json.Marshal(delayedMessage{Stream: stream, Message: message})
	if err != nil {
		return fmt.Errorf("marshal delayed message failed: %w", err)
	}
	member := delayedMember(stream, message.ID)
	pipe := s.delayed.TxPipeline()
	pipe.HSet(context.Background(), delayedPayloadKey, member, payload)
	pipe.ZAdd(context.Background(), delayedScheduleKey, redis.Z{Score: float64(executeAt.UnixMilli()), Member: member})
	_, err = pipe.Exec(context.Background())
	return err
}

// Cancel 取消指定流中尚未转入 Redis Stream 的延迟消息。
func (s *Redis) Cancel(stream string, messageID string) error {
	if stream == "" || messageID == "" {
		return nil
	}
	member := delayedMember(stream, messageID)
	pipe := s.delayed.TxPipeline()
	pipe.ZRem(context.Background(), delayedScheduleKey, member)
	pipe.HDel(context.Background(), delayedPayloadKey, member)
	_, err := pipe.Exec(context.Background())
	return err
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
	s.mux.Lock()
	if s.delayedCancel != nil {
		s.delayedCancel()
	}
	s.mux.Unlock()
	if s.consumer != nil {
		s.consumer.Shutdown()
	}
	s.wait.Wait()
	if s.delayed != nil {
		_ = s.delayed.Close()
	}
}

// runDelayed 周期扫描到期计划，通过短租约保证多实例下只有一个实例负责投递。
func (s *Redis) runDelayed(ctx context.Context) {
	ticker := time.NewTicker(delayedPollPeriod)
	defer ticker.Stop()
	for {
		if err := s.dispatchDue(ctx); err != nil && !errors.Is(err, context.Canceled) {
			// 延迟调度失败不会终止即时队列；计划仍保留并在下一轮重试。
			time.Sleep(delayedPollPeriod)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// dispatchDue 抢占并投递一批到期消息，成功写入 Stream 后才删除计划。
func (s *Redis) dispatchDue(ctx context.Context) error {
	members, err := s.delayed.ZRangeByScore(ctx, delayedScheduleKey, &redis.ZRangeBy{
		Min: "-inf", Max: fmt.Sprintf("%d", time.Now().UnixMilli()), Offset: 0, Count: delayedBatchSize,
	}).Result()
	if err != nil {
		return err
	}
	for _, member := range members {
		if err = s.dispatchDelayedMember(ctx, member); err != nil {
			return err
		}
	}
	return nil
}

// dispatchDelayedMember 使用带过期时间的 Redis 锁抢占单项计划。
func (s *Redis) dispatchDelayedMember(ctx context.Context, member string) error {
	lockKey := delayedLockPrefix + member
	claimed, err := s.delayed.SetNX(ctx, lockKey, "1", delayedLockTTL).Result()
	if err != nil || !claimed {
		return err
	}
	defer s.delayed.Del(context.Background(), lockKey)
	raw, err := s.delayed.HGet(ctx, delayedPayloadKey, member).Bytes()
	if errors.Is(err, redis.Nil) {
		return s.delayed.ZRem(ctx, delayedScheduleKey, member).Err()
	}
	if err != nil {
		return err
	}
	var delayed delayedMessage
	if err = json.Unmarshal(raw, &delayed); err != nil {
		return fmt.Errorf("unmarshal delayed message failed: %w", err)
	}
	// Message.ID 是延迟计划的业务稳定编号，不是 Redis Stream 的毫秒序列编号。
	delayed.Message.ID = ""
	if err = s.Append(delayed.Stream, delayed.Message); err != nil {
		return err
	}
	pipe := s.delayed.TxPipeline()
	pipe.ZRem(ctx, delayedScheduleKey, member)
	pipe.HDel(ctx, delayedPayloadKey, member)
	_, err = pipe.Exec(ctx)
	return err
}

// delayedMember 生成 Redis 延迟计划的稳定成员编号。
func delayedMember(stream string, messageID string) string {
	return stream + "\x00" + messageID
}
