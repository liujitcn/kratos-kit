package mqtt

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/liujitcn/kratos-kit/broker"
)

var (
	// ErrNotConnected 表示 MQTT Broker 尚未连接。
	ErrNotConnected = errors.New("mqtt broker: not connected")
	// ErrConnected 表示 MQTT Broker 已经连接。
	ErrConnected = errors.New("mqtt broker: already connected")
)

type mqttBroker struct {
	mu                 sync.RWMutex
	options            broker.Options
	client             mqtt.Client
	connected          bool
	subscribers        *broker.SubscriberSyncMap
	subscriberID       atomic.Uint64
	statsMu            sync.RWMutex
	lastError          string
	connectedAt        time.Time
	disconnectedAt     time.Time
	published          atomic.Uint64
	publishPending     atomic.Uint64
	maxPublishPending  atomic.Uint64
	publishErrors      atomic.Uint64
	received           atomic.Uint64
	handled            atomic.Uint64
	receiveInFlight    atomic.Uint64
	maxReceiveInFlight atomic.Uint64
	receiveErrors      atomic.Uint64
	subscribeErrors    atomic.Uint64
}

// Stats 描述 MQTT 客户端连接、订阅和消息处理快照。
type Stats struct {
	Connected               bool
	Address                 string
	ClientID                string
	ConnectedAt             time.Time
	DisconnectedAt          time.Time
	RegisteredSubscriptions uint64
	ActiveSubscriptions     uint64
	Published               uint64
	PublishPending          uint64
	MaxPublishPending       uint64
	PublishErrors           uint64
	Received                uint64
	Handled                 uint64
	ReceiveInFlight         uint64
	MaxReceiveInFlight      uint64
	ReceiveErrors           uint64
	SubscribeErrors         uint64
	LastError               string
}

// StatsProvider 提供 MQTT 客户端运行统计快照。
type StatsProvider interface {
	Stats() Stats
}

var _ broker.Broker = (*mqttBroker)(nil)
var _ StatsProvider = (*mqttBroker)(nil)

// NewBroker 创建基于 Paho 的 MQTT 客户端 Broker。
func NewBroker(options ...broker.Option) broker.Broker {
	return &mqttBroker{
		options:     broker.NewOptionsAndApply(options...),
		subscribers: broker.NewSubscriberSyncMap(),
	}
}

// Name 返回 Broker 类型名称。
func (b *mqttBroker) Name() string { return "MQTT" }

// Options 返回 MQTT Broker 配置快照。
func (b *mqttBroker) Options() broker.Options {
	b.mu.RLock()
	defer b.mu.RUnlock()
	options := b.options
	options.Addrs = slices.Clone(options.Addrs)
	options.Tracings = slices.Clone(options.Tracings)
	options.SubscriberMiddlewares = slices.Clone(options.SubscriberMiddlewares)
	options.PublishMiddlewares = slices.Clone(options.PublishMiddlewares)
	return options
}

// Address 返回当前 MQTT Broker 地址。
func (b *mqttBroker) Address() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.options.Addrs) == 0 {
		return "tcp://127.0.0.1:1883"
	}
	return b.options.Addrs[0]
}

// Stats 返回 MQTT 客户端运行统计快照。
func (b *mqttBroker) Stats() Stats {
	options := b.Options()
	b.mu.RLock()
	connected := b.connected
	b.mu.RUnlock()
	b.statsMu.RLock()
	lastError := b.lastError
	connectedAt := b.connectedAt
	disconnectedAt := b.disconnectedAt
	b.statsMu.RUnlock()
	clientID, _ := clientIDFromContext(options.Context)
	return Stats{
		Connected:          connected,
		Address:            b.Address(),
		ClientID:           clientID,
		ConnectedAt:        connectedAt,
		DisconnectedAt:     disconnectedAt,
		Published:          b.published.Load(),
		PublishPending:     b.publishPending.Load(),
		MaxPublishPending:  b.maxPublishPending.Load(),
		PublishErrors:      b.publishErrors.Load(),
		Received:           b.received.Load(),
		Handled:            b.handled.Load(),
		ReceiveInFlight:    b.receiveInFlight.Load(),
		MaxReceiveInFlight: b.maxReceiveInFlight.Load(),
		ReceiveErrors:      b.receiveErrors.Load(),
		SubscribeErrors:    b.subscribeErrors.Load(),
		LastError:          lastError,
	}
}

// Init 初始化 MQTT 客户端配置。
func (b *mqttBroker) Init(options ...broker.Option) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.connected {
		return ErrConnected
	}
	b.options.Apply(options...)
	return nil
}

// Connect 连接 MQTT Broker。
func (b *mqttBroker) Connect() error {
	b.mu.Lock()
	if b.connected {
		b.mu.Unlock()
		return nil
	}
	options := b.options
	clientOptions := mqtt.NewClientOptions()
	applyClientOptions(options, clientOptions)
	clientOptions.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		b.mu.Lock()
		b.connected = false
		b.mu.Unlock()
		b.statsMu.Lock()
		b.disconnectedAt = time.Now()
		b.statsMu.Unlock()
		b.recordError(err)
		if options.ErrorHandler != nil {
			_ = options.ErrorHandler(context.Background(), &connectionEvent{err: err})
		}
	})
	clientOptions.SetOnConnectHandler(func(client mqtt.Client) {
		b.mu.Lock()
		b.connected = true
		b.client = client
		subscribers := b.subscribers.GetAll()
		resumeSubs := true
		if value, ok := resumeSubsFromContext(options.Context); ok {
			resumeSubs = value
		}
		b.mu.Unlock()
		b.statsMu.Lock()
		b.connectedAt = time.Now()
		b.statsMu.Unlock()
		if !resumeSubs {
			return
		}
		for _, item := range subscribers {
			if sub, ok := item.(*mqttSubscriber); ok {
				_ = sub.subscribe(client)
			}
		}
	})
	b.mu.Unlock()

	client := mqtt.NewClient(clientOptions)
	token := client.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		err := errors.New("mqtt broker: connect timeout")
		b.recordError(err)
		return err
	}
	if err := token.Error(); err != nil {
		wrappedErr := fmt.Errorf("mqtt broker: connect: %w", err)
		b.recordError(wrappedErr)
		return wrappedErr
	}
	b.mu.Lock()
	b.client = client
	b.connected = true
	b.mu.Unlock()
	return nil
}

// Disconnect 断开 MQTT Broker 连接。
func (b *mqttBroker) Disconnect() error {
	b.mu.Lock()
	client := b.client
	b.client = nil
	b.connected = false
	b.mu.Unlock()
	b.statsMu.Lock()
	b.disconnectedAt = time.Now()
	b.statsMu.Unlock()
	if client != nil && client.IsConnected() {
		client.Disconnect(1000)
	}
	return nil
}

// Publish 发布 MQTT 消息。
func (b *mqttBroker) Publish(ctx context.Context, topic string, message *broker.Message, opts ...broker.PublishOption) error {
	if message == nil {
		err := errors.New("mqtt broker: message is nil")
		b.recordPublishError(err)
		return err
	}
	b.mu.RLock()
	client := b.client
	codec := b.options.Codec
	b.mu.RUnlock()
	if client == nil || !client.IsConnected() {
		b.recordPublishError(ErrNotConnected)
		return ErrNotConnected
	}
	b.beginPublish()
	publishOptions := broker.NewPublishOptions(opts...)
	var payload []byte
	var err error
	if bytes, ok := message.Body.([]byte); ok {
		payload = bytes
	} else if codec != nil {
		payload, err = codec.Marshal(message.Body)
		if err != nil {
			b.finishPublish()
			b.recordPublishError(err)
			return err
		}
	} else {
		err := errors.New("mqtt broker: codec is nil")
		b.finishPublish()
		b.recordPublishError(err)
		return err
	}
	token := client.Publish(topic, qosFromPublishOptions(publishOptions), false, payload)
	if publishOptions.Async {
		go func() {
			err := token.Error()
			b.finishPublish()
			if err != nil {
				b.recordPublishError(err)
			} else {
				b.published.Add(1)
			}
			if publishOptions.Callback != nil {
				publishOptions.Callback(err)
			}
		}()
		return nil
	}
	defer b.finishPublish()
	if publishOptions.Timeout > 0 && !token.WaitTimeout(publishOptions.Timeout) {
		err := context.DeadlineExceeded
		b.recordPublishError(err)
		return err
	}
	if publishOptions.Timeout <= 0 && !token.Wait() {
		err := context.Canceled
		b.recordPublishError(err)
		return err
	}
	if err = token.Error(); err != nil {
		b.recordPublishError(err)
		return err
	}
	select {
	case <-ctx.Done():
		err := ctx.Err()
		b.recordPublishError(err)
		return err
	default:
		b.published.Add(1)
		return nil
	}
}

// Subscribe 订阅 MQTT 主题。
func (b *mqttBroker) Subscribe(topic string, handler broker.Handler, binder broker.Binder, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	if topic == "" || handler == nil {
		err := errors.New("mqtt broker: topic and handler are required")
		b.subscribeErrors.Add(1)
		b.recordError(err)
		return nil, err
	}
	b.mu.RLock()
	client := b.client
	b.mu.RUnlock()
	if client == nil || !client.IsConnected() {
		b.subscribeErrors.Add(1)
		b.recordError(ErrNotConnected)
		return nil, ErrNotConnected
	}
	options := broker.NewSubscribeOptions(opts...)
	subscriber := &mqttSubscriber{
		owner:   b,
		topic:   topic,
		handler: broker.ChainSubscriberMiddleware(handler, options.Middlewares),
		binder:  binder,
		options: options,
		id:      b.subscriberID.Add(1),
	}
	if err := subscriber.subscribe(client); err != nil {
		b.subscribeErrors.Add(1)
		b.recordError(err)
		return nil, err
	}
	b.subscribers.Add(topic, subscriber)
	return subscriber, nil
}

// Request 不支持 MQTT 请求响应语义。
func (b *mqttBroker) Request(ctx context.Context, topic string, message *broker.Message, opts ...broker.RequestOption) (*broker.Message, error) {
	return nil, fmt.Errorf("%w: %s", broker.ErrRequestUnsupported, topic)
}

type mqttSubscriber struct {
	owner   *mqttBroker
	topic   string
	handler broker.Handler
	binder  broker.Binder
	options broker.SubscribeOptions
	id      uint64
}

func (s *mqttSubscriber) Options() broker.SubscribeOptions { return s.options }
func (s *mqttSubscriber) Topic() string                    { return s.topic }

func (s *mqttSubscriber) subscribe(client mqtt.Client) error {
	token := client.Subscribe(s.topic, qosFromSubscribeOptions(s.options), func(_ mqtt.Client, message mqtt.Message) {
		s.owner.received.Add(1)
		inFlight := s.owner.receiveInFlight.Add(1)
		s.owner.updateMax(&s.owner.maxReceiveInFlight, inFlight)
		defer s.owner.receiveInFlight.Add(^uint64(0))
		event := newMessageEvent(s.topic, message, s.binder, s.owner.Options().Codec)
		ctx := s.options.Context
		if ctx == nil {
			ctx = context.Background()
		}
		if event.err == nil {
			event.err = s.handler(ctx, event)
		}
		if event.err != nil {
			s.owner.receiveErrors.Add(1)
			s.owner.recordError(event.err)
			options := s.owner.Options()
			if options.ErrorHandler != nil {
				_ = options.ErrorHandler(ctx, event)
			}
		} else {
			s.owner.handled.Add(1)
		}
	})
	if !token.WaitTimeout(30 * time.Second) {
		err := errors.New("mqtt broker: subscribe timeout")
		s.owner.recordError(err)
		return err
	}
	if err := token.Error(); err != nil {
		wrappedErr := fmt.Errorf("mqtt broker: subscribe %q: %w", s.topic, err)
		s.owner.recordError(wrappedErr)
		return wrappedErr
	}
	return nil
}

// Unsubscribe 取消 MQTT 主题订阅。
func (s *mqttSubscriber) Unsubscribe(removeFromManager bool) error {
	if s.owner == nil {
		return nil
	}
	s.owner.mu.RLock()
	client := s.owner.client
	s.owner.mu.RUnlock()
	if client != nil && client.IsConnected() {
		token := client.Unsubscribe(s.topic)
		if !token.WaitTimeout(30 * time.Second) {
			return errors.New("mqtt broker: unsubscribe timeout")
		}
		if err := token.Error(); err != nil {
			return err
		}
	}
	if removeFromManager {
		s.owner.subscribers.RemoveOnly(s.topic)
	}
	return nil
}

// beginPublish 记录一个等待 Broker 确认的发布消息。
func (b *mqttBroker) beginPublish() {
	pending := b.publishPending.Add(1)
	b.updateMax(&b.maxPublishPending, pending)
}

// finishPublish 记录一个已结束等待的发布消息。
func (b *mqttBroker) finishPublish() {
	b.publishPending.Add(^uint64(0))
}

// recordPublishError 记录发布失败和最近错误。
func (b *mqttBroker) recordPublishError(err error) {
	if err == nil {
		return
	}
	b.publishErrors.Add(1)
	b.recordError(err)
}

// recordError 更新最近一次 MQTT 错误。
func (b *mqttBroker) recordError(err error) {
	if err == nil {
		return
	}
	b.statsMu.Lock()
	b.lastError = err.Error()
	b.statsMu.Unlock()
}

// updateMax 更新原子峰值计数。
func (b *mqttBroker) updateMax(target *atomic.Uint64, value uint64) {
	for current := target.Load(); value > current; {
		if target.CompareAndSwap(current, value) {
			return
		}
		current = target.Load()
	}
}

type messageEvent struct {
	topic   string
	message *broker.Message
	err     error
}

func newMessageEvent(topic string, raw mqtt.Message, binder broker.Binder, codec interface {
	Unmarshal([]byte, any) error
}) *messageEvent {
	message := broker.NewMessage(nil)
	message.ID = fmt.Sprintf("%d", raw.MessageID())
	message.Metadata["mqtt.qos"] = raw.Qos()
	message.Metadata["mqtt.duplicate"] = raw.Duplicate()
	message.Metadata["mqtt.retained"] = raw.Retained()
	message.Msg = raw
	if binder == nil {
		message.Body = append([]byte(nil), raw.Payload()...)
		return &messageEvent{topic: topic, message: message}
	}
	body := binder()
	if codec == nil {
		return &messageEvent{topic: topic, message: message, err: errors.New("mqtt broker: codec is nil")}
	}
	if err := codec.Unmarshal(raw.Payload(), body); err != nil {
		return &messageEvent{topic: topic, message: message, err: err}
	}
	message.Body = body
	return &messageEvent{topic: topic, message: message}
}

func (e *messageEvent) Topic() string            { return e.topic }
func (e *messageEvent) Message() *broker.Message { return e.message }
func (e *messageEvent) RawMessage() any          { return e.message.Msg }
func (e *messageEvent) Ack() error               { return nil }
func (e *messageEvent) Error() error             { return e.err }

type connectionEvent struct{ err error }

func (e *connectionEvent) Topic() string            { return "_connection" }
func (e *connectionEvent) Message() *broker.Message { return broker.NewMessage(nil) }
func (e *connectionEvent) RawMessage() any          { return nil }
func (e *connectionEvent) Ack() error               { return nil }
func (e *connectionEvent) Error() error             { return e.err }
