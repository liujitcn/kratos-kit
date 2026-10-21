package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	paho "github.com/eclipse/paho.golang/paho"
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
	client             *autopaho.ConnectionManager
	router             *paho.StandardRouter
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
	recorderMu         sync.RWMutex
	recorder           Recorder
}

var _ broker.Broker = (*mqttBroker)(nil)

// NewBroker 创建基于 MQTT 5 Paho 的 MQTT 客户端 Broker。
func NewBroker(options ...broker.Option) broker.Broker {
	return &mqttBroker{options: broker.NewOptionsAndApply(options...), subscribers: broker.NewSubscriberSyncMap()}
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

// Connect 连接 MQTT 5 Broker。
func (b *mqttBroker) Connect() error {
	b.mu.RLock()
	if b.connected {
		b.mu.RUnlock()
		return nil
	}
	options := b.options
	b.mu.RUnlock()
	connectTimeout := 30 * time.Second
	if value, ok := connectTimeoutFromContext(options.Context); ok && value > 0 {
		connectTimeout = value
	}
	router := paho.NewStandardRouter()
	addresses := options.Addrs
	if len(addresses) == 0 {
		addresses = []string{"tcp://127.0.0.1:1883"}
	}
	brokers := make([]*url.URL, 0, len(addresses))
	for _, address := range addresses {
		parsed, err := url.Parse(address)
		if err != nil {
			b.recordError(err)
			return fmt.Errorf("mqtt broker: parse endpoint: %w", err)
		}
		brokers = append(brokers, parsed)
	}
	clientConfig := autopaho.ClientConfig{
		BrokerUrls:        brokers,
		TlsCfg:            options.TLSConfig,
		KeepAlive:         keepAliveSeconds(options),
		ConnectRetryDelay: connectRetryDelay(options),
		ConnectTimeout:    connectTimeout,
		ClientConfig: paho.ClientConfig{
			ClientID: optionsClientID(options),
			Router:   router,
			OnServerDisconnect: func(disconnect *paho.Disconnect) {
				b.markDisconnected()
				b.recordError(fmt.Errorf("mqtt server disconnect: %d", disconnect.ReasonCode))
				if options.ErrorHandler != nil {
					_ = options.ErrorHandler(context.Background(), &connectionEvent{err: fmt.Errorf("mqtt server disconnect: %d", disconnect.ReasonCode)})
				}
			},
			OnClientError: func(clientErr error) {
				b.markDisconnected()
				b.recordError(clientErr)
				if options.ErrorHandler != nil {
					_ = options.ErrorHandler(context.Background(), &connectionEvent{err: clientErr})
				}
			},
		},
		OnConnectionUp: func(manager *autopaho.ConnectionManager, _ *paho.Connack) {
			b.mu.Lock()
			b.client = manager
			b.connected = true
			subscribers := b.subscribers.GetAll()
			b.mu.Unlock()
			resume := true
			if value, ok := resumeSubsFromContext(options.Context); ok {
				resume = value
			}
			if resume {
				for _, item := range subscribers {
					if subscriber, ok := item.(*mqttSubscriber); ok {
						_ = subscriber.subscribe(manager)
					}
				}
			}
		},
		OnConnectError: func(connectErr error) {
			b.recordError(connectErr)
		},
	}
	if auth, ok := authFromContext(options.Context); ok {
		clientConfig.SetUsernamePassword(auth.username, []byte(auth.password))
	}
	manager, err := autopaho.NewConnection(context.Background(), clientConfig)
	if err != nil {
		b.recordError(err)
		return fmt.Errorf("mqtt broker: connect: %w", err)
	}
	connectContext, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	if err = manager.AwaitConnection(connectContext); err != nil {
		b.statsMu.RLock()
		lastError := b.lastError
		b.statsMu.RUnlock()
		if lastError == "" {
			lastError = err.Error()
		}
		return fmt.Errorf("mqtt broker: connect: %w (last attempt: %s)", err, lastError)
	}
	b.mu.Lock()
	b.client = manager
	b.router = router
	b.connected = true
	b.mu.Unlock()
	b.statsMu.Lock()
	b.connectedAt = time.Now()
	b.statsMu.Unlock()
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
	if client != nil {
		return client.Disconnect(context.Background())
	}
	return nil
}

// Publish 发布 MQTT 消息。
func (b *mqttBroker) Publish(ctx context.Context, topic string, message *broker.Message, opts ...broker.PublishOption) error {
	startedAt := time.Now()
	if message == nil {
		err := errors.New("mqtt broker: message is nil")
		b.recordPublishError(err)
		b.recordPublishMessage(ctx, topic, nil, 0, StatusFailed, err, startedAt)
		return err
	}
	b.mu.RLock()
	client := b.client
	connected := b.connected
	codec := b.options.Codec
	b.mu.RUnlock()
	if client == nil || !connected {
		err := ErrNotConnected
		b.recordPublishError(err)
		b.recordPublishMessage(ctx, topic, nil, 0, StatusFailed, err, startedAt)
		return ErrNotConnected
	}
	publishOptions := broker.NewPublishOptions(opts...)
	var payload []byte
	var err error
	if bytes, ok := message.Body.([]byte); ok {
		payload = bytes
	} else if codec != nil {
		payload, err = codec.Marshal(message.Body)
		if err != nil {
			b.recordPublishError(err)
			b.recordPublishMessage(ctx, topic, nil, qosFromPublishOptions(publishOptions), StatusFailed, err, startedAt)
			return err
		}
	} else {
		err = errors.New("mqtt broker: codec is nil")
		b.recordPublishError(err)
		b.recordPublishMessage(ctx, topic, nil, qosFromPublishOptions(publishOptions), StatusFailed, err, startedAt)
		return err
	}
	b.beginPublish()
	defer b.finishPublish()
	response, err := client.Publish(ctx, &paho.Publish{Topic: topic, QoS: qosFromPublishOptions(publishOptions), Payload: payload})
	if err != nil {
		b.recordPublishError(err)
		b.recordPublishMessage(ctx, topic, payload, qosFromPublishOptions(publishOptions), StatusFailed, err, startedAt)
		return err
	}
	if response != nil && response.ReasonCode >= 0x80 {
		err = fmt.Errorf("mqtt broker: publish refused: reason_code=%d", response.ReasonCode)
		b.recordPublishError(err)
		b.recordPublishMessage(ctx, topic, payload, qosFromPublishOptions(publishOptions), StatusFailed, err, startedAt)
		return err
	}
	b.published.Add(1)
	b.recordPublishMessage(ctx, topic, payload, qosFromPublishOptions(publishOptions), StatusSent, nil, startedAt)
	return nil
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
	router := b.router
	connected := b.connected
	b.mu.RUnlock()
	if client == nil || router == nil || !connected {
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
	router.RegisterHandler(topic, subscriber.handle)
	if err := subscriber.subscribe(client); err != nil {
		b.subscribeErrors.Add(1)
		b.recordError(err)
		router.UnregisterHandler(topic)
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

func (s *mqttSubscriber) subscribe(client *autopaho.ConnectionManager) error {
	result, err := client.Subscribe(context.Background(), &paho.Subscribe{Subscriptions: map[string]paho.SubscribeOptions{
		s.topic: {
			QoS:     qosFromSubscribeOptions(s.options),
			NoLocal: noLocalFromSubscribeOptions(s.options),
		},
	}})
	if err != nil {
		return fmt.Errorf("mqtt broker: subscribe %q: %w", s.topic, err)
	}
	if result == nil || len(result.Reasons) == 0 || result.Reasons[0] >= 0x80 {
		return fmt.Errorf("mqtt broker: subscribe %q refused", s.topic)
	}
	return nil
}

func (s *mqttSubscriber) handle(raw *paho.Publish) {
	startedAt := time.Now()
	s.owner.received.Add(1)
	inFlight := s.owner.receiveInFlight.Add(1)
	s.owner.updateMax(&s.owner.maxReceiveInFlight, inFlight)
	defer s.owner.receiveInFlight.Add(^uint64(0))
	event := newMessageEvent(raw.Topic, raw, s.binder, s.owner.Options().Codec)
	ctx := s.options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if event.err == nil {
		event.err = s.handler(ctx, event)
	}
	status := StatusHandled
	if event.err != nil {
		status = StatusFailed
	}
	s.owner.recordSubscribeMessage(ctx, raw, status, event.err, startedAt)
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
}

// Unsubscribe 取消 MQTT 主题订阅。
func (s *mqttSubscriber) Unsubscribe(removeFromManager bool) error {
	if s.owner == nil {
		return nil
	}
	s.owner.mu.RLock()
	client := s.owner.client
	router := s.owner.router
	s.owner.mu.RUnlock()
	if client != nil {
		_, err := client.Unsubscribe(context.Background(), &paho.Unsubscribe{Topics: []string{s.topic}})
		if err != nil {
			return err
		}
	}
	if router != nil {
		router.UnregisterHandler(s.topic)
	}
	if removeFromManager {
		s.owner.subscribers.RemoveOnly(s.topic)
	}
	return nil
}

type messageEvent struct {
	topic   string
	message *broker.Message
	err     error
}

func newMessageEvent(topic string, raw *paho.Publish, binder broker.Binder, codec interface {
	Unmarshal([]byte, any) error
}) *messageEvent {
	message := broker.NewMessage(nil)
	message.ID = fmt.Sprintf("%d", raw.PacketID)
	message.Metadata["mqtt.qos"] = raw.QoS
	message.Metadata["mqtt.retained"] = raw.Retain
	message.Msg = raw
	if binder == nil {
		message.Body = append([]byte(nil), raw.Payload...)
		return &messageEvent{topic: topic, message: message}
	}
	body := binder()
	if codec == nil {
		return &messageEvent{topic: topic, message: message, err: errors.New("mqtt broker: codec is nil")}
	}
	if err := codec.Unmarshal(raw.Payload, body); err != nil {
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

func optionsClientID(options broker.Options) string {
	if value, ok := clientIDFromContext(options.Context); ok && value != "" {
		return value
	}
	return fmt.Sprintf("kratos-mqtt-%d", time.Now().UnixNano())
}

func keepAliveSeconds(options broker.Options) uint16 {
	if value, ok := keepAliveFromContext(options.Context); ok && value > 0 {
		return uint16(value / time.Second)
	}
	return 30
}

func cleanSession(options broker.Options) bool {
	if value, ok := cleanSessionFromContext(options.Context); ok {
		return value
	}
	return true
}

func connectRetryDelay(options broker.Options) time.Duration {
	if value, ok := connectRetryIntervalFromContext(options.Context); ok && value > 0 {
		return value
	}
	return 5 * time.Second
}

func (b *mqttBroker) markDisconnected() {
	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()
	b.statsMu.Lock()
	b.disconnectedAt = time.Now()
	b.statsMu.Unlock()
}
