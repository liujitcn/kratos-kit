package redisqueue

import (
	"context"
	stderrors "errors"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// ConsumerFunc 消息消费处理函数。
type ConsumerFunc func(*Message) error

type registeredConsumer struct {
	fn ConsumerFunc
	id string
}

// ConsumerOptions 消费者配置。
type ConsumerOptions struct {
	Name              string
	GroupName         string
	VisibilityTimeout time.Duration
	BlockingTimeout   time.Duration
	ReclaimInterval   time.Duration
	BufferSize        int
	Concurrency       int
	RedisOptions      *RedisOptions
}

// Consumer Redis Stream 消费器。
type Consumer struct {
	// Errors 用于上报消费过程中的错误；通道带缓冲，避免无人监听时阻塞主流程。
	Errors chan error

	options   *ConsumerOptions
	redis     redis.UniversalClient
	consumers map[string]registeredConsumer
	queue     chan *Message
	wg        *sync.WaitGroup
	bgWg      *sync.WaitGroup

	mux      sync.RWMutex
	running  atomic.Bool
	stopOnce sync.Once
	stopCh   chan struct{}
}

var defaultConsumerOptions = &ConsumerOptions{
	VisibilityTimeout: 60 * time.Second,
	BlockingTimeout:   5 * time.Second,
	ReclaimInterval:   1 * time.Second,
	BufferSize:        100,
	Concurrency:       10,
}

// NewConsumer 使用默认配置创建消费者。
func NewConsumer() (*Consumer, error) {
	return NewConsumerWithOptions(defaultConsumerOptions)
}

// normalizeConsumerOptions 归一化消费者配置并补齐默认值。
func normalizeConsumerOptions(options *ConsumerOptions) *ConsumerOptions {
	if options == nil {
		options = &ConsumerOptions{}
	}

	hostname, _ := os.Hostname()
	if options.Name == "" {
		options.Name = hostname
	}
	if options.GroupName == "" {
		options.GroupName = "redisqueue"
	}
	if options.VisibilityTimeout == 0 {
		options.VisibilityTimeout = defaultConsumerOptions.VisibilityTimeout
	}
	if options.BlockingTimeout == 0 {
		options.BlockingTimeout = defaultConsumerOptions.BlockingTimeout
	}
	if options.ReclaimInterval == 0 {
		options.ReclaimInterval = defaultConsumerOptions.ReclaimInterval
	}
	if options.BufferSize <= 0 {
		options.BufferSize = defaultConsumerOptions.BufferSize
	}
	if options.Concurrency <= 0 {
		options.Concurrency = defaultConsumerOptions.Concurrency
	}

	return options
}

// NewConsumerWithOptions 使用自定义配置创建消费者。
func NewConsumerWithOptions(options *ConsumerOptions) (*Consumer, error) {
	options = normalizeConsumerOptions(options)

	redisClient, err := newCheckedRedisClient(options.RedisOptions)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		Errors: make(chan error, options.BufferSize),

		options:   options,
		redis:     redisClient,
		consumers: make(map[string]registeredConsumer),
		queue:     make(chan *Message, options.BufferSize),
		wg:        &sync.WaitGroup{},
		bgWg:      &sync.WaitGroup{},
		stopCh:    make(chan struct{}),
	}, nil
}

// RegisterWithLastID 注册流消费处理函数，并指定首次建组时的起始消息 ID。
func (c *Consumer) RegisterWithLastID(stream string, id string, fn ConsumerFunc) {
	if len(id) == 0 {
		id = "0"
	}

	c.mux.Lock()
	c.consumers[stream] = registeredConsumer{
		fn: fn,
		id: id,
	}
	running := c.running.Load()
	c.mux.Unlock()

	if running {
		if err := c.createConsumerGroup(stream, id); err != nil {
			c.reportError(errors.Wrap(err, "error creating consumer group"))
		}
	}
}

// Register 注册流消费处理函数。
func (c *Consumer) Register(stream string, fn ConsumerFunc) {
	c.RegisterWithLastID(stream, "0", fn)
}

// Run 启动消费者并阻塞，直到收到关闭信号。
func (c *Consumer) Run() {
	if !c.running.CompareAndSwap(false, true) {
		c.wg.Wait()
		return
	}

	consumers := c.snapshotConsumers()
	if len(consumers) == 0 {
		c.running.Store(false)
		c.reportError(errors.New("at least one consumer function needs to be registered"))
		return
	}
	if err := c.prepareConsumerGroups(consumers); err != nil {
		c.running.Store(false)
		c.reportError(err)
		return
	}

	if c.options.VisibilityTimeout > 0 {
		c.bgWg.Add(1)
		go c.reclaim()
	}
	c.bgWg.Add(1)
	go c.poll()
	go func() {
		c.bgWg.Wait()
		close(c.queue)
	}()

	c.wg.Add(c.options.Concurrency)
	for i := 0; i < c.options.Concurrency; i++ {
		go c.work()
	}

	c.wg.Wait()
}

// Shutdown 停止拉取新消息，并等待在途消息处理完成。
func (c *Consumer) Shutdown() {
	if !c.running.CompareAndSwap(true, false) {
		return
	}

	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// prepareConsumerGroups 为当前所有已注册流准备消费组。
func (c *Consumer) prepareConsumerGroups(consumers map[string]registeredConsumer) error {
	for stream, consumer := range consumers {
		if err := c.createConsumerGroup(stream, consumer.id); err != nil {
			return errors.Wrap(err, "error creating consumer group")
		}
	}

	return nil
}

// createConsumerGroup 为指定流创建消费组。
func (c *Consumer) createConsumerGroup(stream string, id string) error {
	err := c.redis.XGroupCreateMkStream(context.TODO(), stream, c.options.GroupName, id).Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}

	return nil
}

// snapshotConsumers 获取已注册消费者快照。
func (c *Consumer) snapshotConsumers() map[string]registeredConsumer {
	c.mux.RLock()
	defer c.mux.RUnlock()

	consumers := make(map[string]registeredConsumer, len(c.consumers))
	for stream, consumer := range c.consumers {
		consumers[stream] = consumer
	}

	return consumers
}

// snapshotStreamNames 获取已注册流名称快照。
func (c *Consumer) snapshotStreamNames() []string {
	c.mux.RLock()
	defer c.mux.RUnlock()

	streams := make([]string, 0, len(c.consumers))
	for stream := range c.consumers {
		streams = append(streams, stream)
	}

	return streams
}

// buildReadStreams 构造 XREADGROUP 所需的流参数。
func (c *Consumer) buildReadStreams() []string {
	streams := c.snapshotStreamNames()
	if len(streams) == 0 {
		return nil
	}

	readStreams := make([]string, 0, len(streams)*2)
	readStreams = append(readStreams, streams...)
	for range streams {
		readStreams = append(readStreams, ">")
	}

	return readStreams
}

// getConsumer 获取指定流的消费处理函数。
func (c *Consumer) getConsumer(stream string) (registeredConsumer, bool) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	consumer, ok := c.consumers[stream]
	return consumer, ok
}

// reportError 以非阻塞方式上报错误，避免消费主流程被错误通道卡住。
func (c *Consumer) reportError(err error) {
	if err == nil {
		return
	}

	select {
	case c.Errors <- err:
	default:
	}
}

// reclaim 检查并回收超时未确认的消息。
func (c *Consumer) reclaim() {
	defer c.bgWg.Done()

	if c.options.VisibilityTimeout == 0 {
		return
	}

	ticker := time.NewTicker(c.options.ReclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			for _, stream := range c.snapshotStreamNames() {
				start := "-"
				end := "+"

				for {
					res, err := c.redis.XPendingExt(context.TODO(), &redis.XPendingExtArgs{
						Stream: stream,
						Group:  c.options.GroupName,
						Start:  start,
						End:    end,
						Count:  int64(c.options.BufferSize - len(c.queue)),
					}).Result()
					if err != nil && !stderrors.Is(err, redis.Nil) {
						c.reportError(errors.Wrap(err, "error listing pending messages"))
						break
					}
					if len(res) == 0 {
						break
					}

					for _, r := range res {
						if r.Idle < c.options.VisibilityTimeout {
							continue
						}

						// 显式声明返回值，避免与上文 err 形成混合短声明。
						var claimres []redis.XMessage
						claimres, err = c.redis.XClaim(context.TODO(), &redis.XClaimArgs{
							Stream:   stream,
							Group:    c.options.GroupName,
							Consumer: c.options.Name,
							MinIdle:  c.options.VisibilityTimeout,
							Messages: []string{r.ID},
						}).Result()
						if err != nil && !stderrors.Is(err, redis.Nil) {
							c.reportError(errors.Wrap(err, "error claiming message"))
							break
						}
						// 消息已经被裁剪或删除时，需要主动 ack 清理 pending 状态。
						if stderrors.Is(err, redis.Nil) {
							err = c.redis.XAck(context.TODO(), stream, c.options.GroupName, r.ID).Err()
							if err != nil {
								c.reportError(errors.Wrapf(err, "error acknowledging after failed claim for %q stream and %q message", stream, r.ID))
							}
							continue
						}

						c.enqueue(stream, claimres)
					}

					// 显式声明新消息游标，避免复用 err 时触发混合短声明告警。
					var newID string
					newID, err = incrementMessageID(res[len(res)-1].ID)
					if err != nil {
						c.reportError(err)
						break
					}
					start = newID
				}
			}
		}
	}
}

// poll 轮询 Redis Stream 新消息。
func (c *Consumer) poll() {
	defer c.bgWg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		default:
			res, err := c.redis.XReadGroup(context.TODO(), &redis.XReadGroupArgs{
				Group:    c.options.GroupName,
				Consumer: c.options.Name,
				Streams:  c.buildReadStreams(),
				Count:    int64(c.options.BufferSize - len(c.queue)),
				Block:    c.options.BlockingTimeout,
			}).Result()
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					continue
				}
				if stderrors.Is(err, redis.Nil) {
					continue
				}
				c.reportError(errors.Wrap(err, "error reading redis stream"))
				continue
			}

			for _, r := range res {
				c.enqueue(r.Stream, r.Messages)
			}
		}
	}
}

// enqueue 投递消息到本地工作队列。
func (c *Consumer) enqueue(stream string, msgs []redis.XMessage) {
	for _, m := range msgs {
		msg := &Message{
			ID:     m.ID,
			Stream: stream,
			Values: m.Values,
		}
		c.queue <- msg
	}
}

// work 工作协程负责处理消息并执行确认。
func (c *Consumer) work() {
	defer c.wg.Done()

	for msg := range c.queue {
		err := c.process(msg)
		if err != nil {
			c.reportError(errors.Wrapf(err, "error calling ConsumerFunc for %q stream and %q message", msg.Stream, msg.ID))
			continue
		}

		err = c.redis.XAck(context.TODO(), msg.Stream, c.options.GroupName, msg.ID).Err()
		if err != nil {
			c.reportError(errors.Wrapf(err, "error acknowledging after success for %q stream and %q message", msg.Stream, msg.ID))
			continue
		}
	}
}

// process 执行具体的消费处理函数。
func (c *Consumer) process(msg *Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = errors.Wrap(e, "ConsumerFunc panic")
				return
			}
			err = errors.Errorf("ConsumerFunc panic: %v", r)
		}
	}()

	consumer, ok := c.getConsumer(msg.Stream)
	if !ok {
		return errors.Errorf("consumer for %q stream not found", msg.Stream)
	}

	err = consumer.fn(msg)
	return
}
