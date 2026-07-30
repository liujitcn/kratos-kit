package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/trace"

	"github.com/liujitcn/kratos-kit/broker"
	"github.com/liujitcn/kratos-kit/tracing"
)

const (
	defaultAddress   = nats.DefaultURL
	coreSystemName   = "nats"
	jetStreamName    = "nats-jetstream"
	defaultPullBatch = 10
	defaultPullWait  = time.Second
	drainCloseGrace  = 6 * time.Second
	producerSpanName = "nats publish"
	consumerSpanName = "nats receive"
)

var (
	// ErrNotConnected 表示 broker 尚未连接。
	ErrNotConnected = errors.New("nats broker: not connected")
	// ErrConnected 表示连接期间不能重新初始化 broker。
	ErrConnected = errors.New("nats broker: already connected")
)

type natsBroker struct {
	mu sync.RWMutex

	options      broker.Options
	natsOptions  nats.Options
	conn         *nats.Conn
	jetStream    nats.JetStreamContext
	useJetStream bool
	drain        bool

	subscribers    *broker.SubscriberSyncMap
	subscriberID   atomic.Uint64
	producerTracer *tracing.Tracer
	consumerTracer *tracing.Tracer
}

var _ broker.Broker = (*natsBroker)(nil)
var _ subscriberRemover = (*natsBroker)(nil)

// NewBroker 创建 Core NATS broker。
func NewBroker(options ...broker.Option) broker.Broker {
	return newBroker(false, options...)
}

// NewJetStreamBroker 创建 NATS JetStream broker。
func NewJetStreamBroker(options ...broker.Option) broker.Broker {
	return newBroker(true, options...)
}

// newBroker 创建共享实现，并仅在 SDK 发布订阅位置区分 Core 与 JetStream。
func newBroker(useJetStream bool, options ...broker.Option) *natsBroker {
	instance := &natsBroker{
		options:      broker.NewOptionsAndApply(options...),
		useJetStream: useJetStream,
		subscribers:  broker.NewSubscriberSyncMap(),
	}
	instance.configureLocked()
	return instance
}

// Name 返回 broker 类型名称。
func (b *natsBroker) Name() string {
	if b.useJetStream {
		return "NATS-JetStream"
	}
	return "NATS"
}

// Options 返回 broker 公共配置的快照。
func (b *natsBroker) Options() broker.Options {
	b.mu.RLock()
	defer b.mu.RUnlock()

	options := b.options
	options.Addrs = append([]string(nil), b.options.Addrs...)
	options.Tracings = append([]tracing.Option(nil), b.options.Tracings...)
	options.SubscriberMiddlewares = append([]broker.SubscriberMiddleware(nil), b.options.SubscriberMiddlewares...)
	options.PublishMiddlewares = append([]broker.PublishMiddleware(nil), b.options.PublishMiddlewares...)
	return options
}

// Address 返回当前连接地址；未连接时返回首个配置地址。
func (b *natsBroker) Address() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn != nil && b.conn.IsConnected() {
		return b.conn.ConnectedUrl()
	}
	if len(b.options.Addrs) > 0 {
		return b.options.Addrs[0]
	}
	return defaultAddress
}

// Init 在建立连接前更新公共配置与 NATS SDK 选项。
func (b *natsBroker) Init(options ...broker.Option) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil && !b.conn.IsClosed() {
		return ErrConnected
	}
	b.options.Apply(options...)
	b.configureLocked()
	return nil
}

// configureLocked 合并公共配置与原生 NATS 选项。
func (b *natsBroker) configureLocked() {
	natsOptions := nats.GetDefaultOptions()
	if configured, ok := contextValue[nats.Options](b.options.Context, optionsKey{}); ok {
		natsOptions = configured
	}

	addresses := b.options.Addrs
	if len(addresses) == 0 {
		addresses = natsOptions.Servers
	}
	b.options.Addrs = normalizeAddresses(addresses)
	natsOptions.Servers = append([]string(nil), b.options.Addrs...)
	if b.options.Secure {
		natsOptions.Secure = true
	}
	if b.options.TLSConfig != nil {
		natsOptions.Secure = true
		natsOptions.TLSConfig = b.options.TLSConfig
	}

	b.natsOptions = natsOptions
	b.drain, _ = contextValue[bool](b.options.Context, drainConnectionKey{})
	b.producerTracer = nil
	b.consumerTracer = nil
	if len(b.options.Tracings) > 0 {
		b.producerTracer = tracing.NewTracer(trace.SpanKindProducer, producerSpanName, b.options.Tracings...)
		b.consumerTracer = tracing.NewTracer(trace.SpanKindConsumer, consumerSpanName, b.options.Tracings...)
	}
}

// normalizeAddresses 补全 NATS scheme 并过滤空地址。
func normalizeAddresses(addresses []string) []string {
	normalized := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address == "" {
			continue
		}
		if !strings.Contains(address, "://") {
			address = "nats://" + address
		}
		normalized = append(normalized, address)
	}
	if len(normalized) == 0 {
		return []string{defaultAddress}
	}
	return normalized
}

// Connect 建立 NATS 连接，并按需创建 JetStream context。
func (b *natsBroker) Connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil {
		switch b.conn.Status() {
		case nats.CONNECTED, nats.CONNECTING, nats.RECONNECTING:
			return nil
		}
	}

	connection, err := b.natsOptions.Connect()
	if err != nil {
		return fmt.Errorf("连接 NATS: %w", err)
	}

	if b.useJetStream {
		contextOptions, _ := contextValue[[]nats.JSOpt](b.options.Context, jetStreamContextOptionsKey{})
		var jetStream nats.JetStreamContext
		jetStream, err = connection.JetStream(contextOptions...)
		if err != nil {
			connection.Close()
			return fmt.Errorf("创建 JetStream context: %w", err)
		}
		b.jetStream = jetStream
	}
	b.conn = connection
	return nil
}

// Disconnect 停止订阅并关闭或排空 NATS 连接。
func (b *natsBroker) Disconnect() error {
	b.mu.Lock()
	connection := b.conn
	drain := b.drain
	b.conn = nil
	b.jetStream = nil
	b.mu.Unlock()

	if connection == nil {
		b.subscribers.Clear()
		return nil
	}
	if !drain {
		b.subscribers.Clear()
		connection.Close()
		return nil
	}

	closed := connection.StatusChanged(nats.CLOSED)
	var err error
	if err = connection.Drain(); err != nil {
		connection.RemoveStatusListener(closed)
		b.subscribers.Clear()
		connection.Close()
		if errors.Is(err, nats.ErrConnectionClosed) {
			return nil
		}
		return fmt.Errorf("排空 NATS 连接: %w", err)
	}
	drainTimeout := connection.Opts.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = nats.DefaultDrainTimeout
	}
	timer := time.NewTimer(drainTimeout + drainCloseGrace)
	select {
	case <-closed:
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	case <-timer.C:
		connection.RemoveStatusListener(closed)
		connection.Close()
		b.subscribers.Clear()
		return fmt.Errorf("等待 NATS 连接排空: %w", nats.ErrDrainTimeout)
	}
	connection.RemoveStatusListener(closed)
	// SDK 已完成订阅排空，此处仅同步本地订阅状态，不会丢弃待处理消息。
	b.subscribers.Clear()
	if err = connection.LastError(); errors.Is(err, nats.ErrDrainTimeout) {
		return fmt.Errorf("排空 NATS 连接: %w", err)
	}
	return nil
}

// Publish 编码并发布消息，同时执行公共发布 middleware。
func (b *natsBroker) Publish(ctx context.Context, topic string, message *broker.Message, options ...broker.PublishOption) error {
	handler := b.publish
	if len(b.options.PublishMiddlewares) > 0 {
		handler = broker.ChainPublishMiddleware(handler, b.options.PublishMiddlewares)
	}
	return handler(ctx, topic, message, options...)
}

// publish 执行一次 Core NATS 或 JetStream 发布。
func (b *natsBroker) publish(ctx context.Context, topic string, message *broker.Message, options ...broker.PublishOption) error {
	if message == nil {
		return errors.New("nats broker: message is nil")
	}

	publishOptions := broker.NewPublishOptions(broker.WithPublishContext(ctx))
	publishOptions.Apply(options...)
	publishCtx := normalizeContext(publishOptions.Context)
	var cancel context.CancelFunc
	if publishOptions.Timeout > 0 {
		publishCtx, cancel = context.WithTimeout(publishCtx, publishOptions.Timeout)
		defer cancel()
	}

	payload, err := broker.Marshal(b.options.Codec, message.Body)
	if err != nil {
		return fmt.Errorf("编码 NATS 消息: %w", err)
	}
	natsMessage := nats.NewMsg(topic)
	natsMessage.Data = payload
	applyMessageHeaders(natsMessage, message.Headers)
	if headers, ok := contextValue[map[string][]string](publishOptions.Context, headersKey{}); ok {
		applyHeaders(natsMessage, headers)
	}

	b.mu.RLock()
	connection := b.conn
	jetStream := b.jetStream
	useJetStream := b.useJetStream
	producerTracer := b.producerTracer
	b.mu.RUnlock()
	if connection == nil || connection.IsClosed() {
		return ErrNotConnected
	}

	system := coreSystemName
	if useJetStream {
		system = jetStreamName
	}
	var span trace.Span
	publishCtx, span = startProducerSpan(publishCtx, producerTracer, system, natsMessage)
	if useJetStream {
		if jetStream == nil {
			err = ErrNotConnected
		} else {
			_, err = jetStream.PublishMsg(natsMessage, buildPublishOptions(publishCtx, publishOptions)...)
		}
	} else {
		err = connection.PublishMsg(natsMessage)
		if err == nil && publishOptions.Timeout > 0 {
			err = connection.FlushWithContext(publishCtx)
		}
	}
	finishSpan(publishCtx, producerTracer, span, err)
	if publishOptions.Callback != nil {
		publishOptions.Callback(err)
	}
	if err != nil {
		return fmt.Errorf("发布 NATS 消息: %w", err)
	}
	return nil
}

// buildPublishOptions 将公共发布上下文转换为 JetStream SDK 选项。
func buildPublishOptions(ctx context.Context, options broker.PublishOptions) []nats.PubOpt {
	result := []nats.PubOpt{nats.Context(ctx)}
	if value, ok := contextValue[string](options.Context, publishMessageIDKey{}); ok && value != "" {
		result = append(result, nats.MsgId(value))
	}
	if value, ok := contextValue[string](options.Context, publishExpectedStreamKey{}); ok && value != "" {
		result = append(result, nats.ExpectStream(value))
	}
	if value, ok := contextValue[uint64](options.Context, publishExpectedLastSequenceKey{}); ok {
		result = append(result, nats.ExpectLastSequence(value))
	}
	if value, ok := contextValue[uint64](options.Context, publishExpectedLastSequencePerSubjectKey{}); ok {
		result = append(result, nats.ExpectLastSequencePerSubject(value))
	}
	if value, ok := contextValue[string](options.Context, publishExpectedLastMessageIDKey{}); ok && value != "" {
		result = append(result, nats.ExpectLastMsgId(value))
	}
	if raw, ok := contextValue[[]nats.PubOpt](options.Context, publishRawOptionsKey{}); ok {
		result = append(result, raw...)
	}
	return result
}

// Subscribe 创建 Core NATS 或 JetStream 订阅。
func (b *natsBroker) Subscribe(topic string, handler broker.Handler, binder broker.Binder, options ...broker.SubscribeOption) (broker.Subscriber, error) {
	if handler == nil {
		return nil, errors.New("nats broker: handler is nil")
	}

	subscribeOptions := broker.NewSubscribeOptions(options...)
	subscribeOptions.Context = normalizeContext(subscribeOptions.Context)
	middlewares := append([]broker.SubscriberMiddleware(nil), b.options.SubscriberMiddlewares...)
	middlewares = append(middlewares, subscribeOptions.Middlewares...)
	handler = broker.ChainSubscriberMiddleware(handler, middlewares)

	b.mu.RLock()
	connection := b.conn
	jetStream := b.jetStream
	useJetStream := b.useJetStream
	b.mu.RUnlock()
	if connection == nil || connection.IsClosed() || useJetStream && jetStream == nil {
		return nil, ErrNotConnected
	}

	key := fmt.Sprintf("%s:%d", topic, b.subscriberID.Add(1))
	managed := &subscriber{
		remover: b,
		key:     key,
		topic:   topic,
		options: subscribeOptions,
	}
	callback := func(message *nats.Msg) {
		b.handleMessage(subscribeOptions, handler, binder, message)
	}

	var subscription *nats.Subscription
	var err error
	if useJetStream {
		subscription, err = b.subscribeJetStream(topic, managed, callback, subscribeOptions)
	} else if subscribeOptions.Queue != "" {
		subscription, err = connection.QueueSubscribe(topic, subscribeOptions.Queue, callback)
	} else {
		subscription, err = connection.Subscribe(topic, callback)
	}
	if err != nil {
		return nil, fmt.Errorf("订阅 NATS subject %q: %w", topic, err)
	}
	managed.sub = subscription
	b.subscribers.Add(key, managed)
	return managed, nil
}

// subscribeJetStream 创建 push 或 pull JetStream consumer。
func (b *natsBroker) subscribeJetStream(
	topic string,
	managed *subscriber,
	callback func(*nats.Msg),
	options broker.SubscribeOptions,
) (*nats.Subscription, error) {
	subscriptionOptions := buildSubscribeOptions(options)
	pull, _ := contextValue[bool](options.Context, subscribePullKey{})
	if !pull {
		if options.Queue != "" {
			return b.jetStream.QueueSubscribe(topic, options.Queue, callback, subscriptionOptions...)
		}
		return b.jetStream.Subscribe(topic, callback, subscriptionOptions...)
	}

	durable, _ := contextValue[string](options.Context, subscribeDurableKey{})
	subscription, err := b.jetStream.PullSubscribe(topic, durable, subscriptionOptions...)
	if err != nil {
		return nil, err
	}
	batchSize := defaultPullBatch
	if configured, ok := contextValue[int](options.Context, subscribePullBatchSizeKey{}); ok && configured > 0 {
		batchSize = configured
	}
	go b.pullLoop(managed, subscription, callback, batchSize)
	return subscription, nil
}

// buildSubscribeOptions 将公共订阅上下文转换为 JetStream SDK 选项。
func buildSubscribeOptions(options broker.SubscribeOptions) []nats.SubOpt {
	var result []nats.SubOpt
	if value, ok := contextValue[string](options.Context, subscribeDurableKey{}); ok && value != "" {
		result = append(result, nats.Durable(value))
	}
	if value, _ := contextValue[bool](options.Context, subscribeDeliverAllKey{}); value {
		result = append(result, nats.DeliverAll())
	}
	if value, _ := contextValue[bool](options.Context, subscribeDeliverLastKey{}); value {
		result = append(result, nats.DeliverLast())
	}
	if value, _ := contextValue[bool](options.Context, subscribeDeliverNewKey{}); value {
		result = append(result, nats.DeliverNew())
	}
	if value, ok := contextValue[uint64](options.Context, subscribeStartSequenceKey{}); ok {
		result = append(result, nats.StartSequence(value))
	}
	if value, ok := contextValue[time.Time](options.Context, subscribeStartTimeKey{}); ok && !value.IsZero() {
		result = append(result, nats.StartTime(value))
	}
	if value, ok := contextValue[time.Duration](options.Context, subscribeAckWaitKey{}); ok && value > 0 {
		result = append(result, nats.AckWait(value))
	}
	if value, ok := contextValue[int](options.Context, subscribeMaxAckPendingKey{}); ok && value > 0 {
		result = append(result, nats.MaxAckPending(value))
	}
	if value, ok := contextValue[string](options.Context, subscribeBindStreamKey{}); ok && value != "" {
		result = append(result, nats.BindStream(value))
	}
	if value, _ := contextValue[bool](options.Context, subscribeReplayInstantKey{}); value {
		result = append(result, nats.ReplayInstant())
	}
	if value, ok := contextValue[string](options.Context, subscribeDescriptionKey{}); ok && value != "" {
		result = append(result, nats.Description(value))
	}
	manual, _ := contextValue[bool](options.Context, subscribeManualAckKey{})
	if manual || !options.AutoAck {
		result = append(result, nats.ManualAck())
	}
	if raw, ok := contextValue[[]nats.SubOpt](options.Context, subscribeRawOptionsKey{}); ok {
		result = append(result, raw...)
	}
	return result
}

// pullLoop 持续拉取 JetStream 消息，订阅关闭后退出。
func (b *natsBroker) pullLoop(managed *subscriber, subscription *nats.Subscription, callback func(*nats.Msg), batchSize int) {
	for !managed.IsClosed() {
		messages, err := subscription.Fetch(batchSize, nats.MaxWait(defaultPullWait))
		if err != nil {
			if managed.IsClosed() || errors.Is(err, nats.ErrConnectionClosed) {
				return
			}
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			time.Sleep(defaultPullWait)
			continue
		}
		for _, message := range messages {
			callback(message)
		}
	}
}

// handleMessage 解码并处理一条消息，统一错误回调、重试、ack 和 tracing 行为。
func (b *natsBroker) handleMessage(options broker.SubscribeOptions, handler broker.Handler, binder broker.Binder, source *nats.Msg) {
	message := broker.NewMessage(nil, broker.WithHeaders(toBrokerHeaders(source.Header)), broker.WithMsg(source))
	publication := &publication{
		topic:      source.Subject,
		message:    message,
		ackEnabled: b.useJetStream,
	}

	ctx := normalizeContext(options.Context)
	system := coreSystemName
	if b.useJetStream {
		system = jetStreamName
	}
	var span trace.Span
	ctx, span = startConsumerSpan(ctx, b.consumerTracer, system, source)

	var err error
	if binder == nil {
		message.Body = append([]byte(nil), source.Data...)
	} else {
		message.Body = binder()
		if message.Body == nil {
			err = errors.New("NATS binder 返回 nil")
		} else {
			err = broker.Unmarshal(b.options.Codec, source.Data, message.Body)
		}
	}
	if err == nil {
		for attempt := 0; attempt <= options.MaxRetries; attempt++ {
			err = handler(ctx, publication)
			if err == nil || attempt == options.MaxRetries {
				break
			}
			if options.RetryDelay > 0 {
				timer := time.NewTimer(options.RetryDelay)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					err = ctx.Err()
				case <-timer.C:
				}
				if err != nil && errors.Is(err, ctx.Err()) {
					break
				}
			}
		}
	}

	if err != nil {
		publication.err = err
		if b.options.ErrorHandler != nil {
			_ = b.options.ErrorHandler(ctx, publication)
		}
		if b.useJetStream {
			_ = source.Nak()
		}
		finishSpan(ctx, b.consumerTracer, span, err)
		return
	}
	if options.AutoAck && b.useJetStream {
		err = publication.Ack()
	}
	finishSpan(ctx, b.consumerTracer, span, err)
}

// Request 通过 Core NATS request/reply 发送请求并等待响应。
func (b *natsBroker) Request(ctx context.Context, topic string, message *broker.Message, options ...broker.RequestOption) (*broker.Message, error) {
	if message == nil {
		return nil, errors.New("nats broker: message is nil")
	}

	requestOptions := broker.NewRequestOptions(broker.WithRequestContext(ctx))
	requestOptions.Apply(options...)
	requestCtx := normalizeContext(requestOptions.Context)
	var cancel context.CancelFunc
	if requestOptions.Timeout > 0 {
		requestCtx, cancel = context.WithTimeout(requestCtx, requestOptions.Timeout)
		defer cancel()
	}

	payload, err := broker.Marshal(b.options.Codec, message.Body)
	if err != nil {
		return nil, fmt.Errorf("编码 NATS 请求: %w", err)
	}
	natsMessage := nats.NewMsg(topic)
	natsMessage.Data = payload
	applyMessageHeaders(natsMessage, message.Headers)
	if headers, ok := contextValue[map[string][]string](requestOptions.Context, headersKey{}); ok {
		applyHeaders(natsMessage, headers)
	}

	b.mu.RLock()
	connection := b.conn
	producerTracer := b.producerTracer
	b.mu.RUnlock()
	if connection == nil || connection.IsClosed() {
		return nil, ErrNotConnected
	}

	var span trace.Span
	requestCtx, span = startProducerSpan(requestCtx, producerTracer, coreSystemName, natsMessage)
	var response *nats.Msg
	if requestOptions.ReplyTopic == "" {
		response, err = connection.RequestMsgWithContext(requestCtx, natsMessage)
	} else {
		response, err = requestWithReplyTopic(requestCtx, connection, natsMessage, requestOptions.ReplyTopic)
	}
	finishSpan(requestCtx, producerTracer, span, err)
	if err != nil {
		return nil, fmt.Errorf("请求 NATS subject %q: %w", topic, err)
	}
	return broker.NewMessage(
		append([]byte(nil), response.Data...),
		broker.WithHeaders(toBrokerHeaders(response.Header)),
		broker.WithMsg(response),
	), nil
}

// requestWithReplyTopic 从调用方指定的 reply 前缀派生唯一 subject 完成一次请求。
func requestWithReplyTopic(ctx context.Context, connection *nats.Conn, message *nats.Msg, replyTopic string) (*nats.Msg, error) {
	replySubject := replyTopic + "." + nats.NewInbox()
	subscription, err := connection.SubscribeSync(replySubject)
	if err != nil {
		return nil, err
	}
	defer subscription.Unsubscribe()

	message.Reply = replySubject
	if err = connection.PublishMsg(message); err != nil {
		return nil, err
	}
	return subscription.NextMsgWithContext(ctx)
}

// Conn 返回底层 NATS 连接。
func (b *natsBroker) Conn() *nats.Conn {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.conn
}

// JetStreamContext 返回底层 JetStream context。
func (b *natsBroker) JetStreamContext() nats.JetStreamContext {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.jetStream
}

// removeSubscriber 从 broker 的订阅管理器移除指定订阅。
func (b *natsBroker) removeSubscriber(key string) bool {
	return b.subscribers.RemoveOnly(key)
}

// GetConn 从公共 broker 接口提取底层 NATS 连接。
func GetConn(instance broker.Broker) *nats.Conn {
	accessor, ok := instance.(interface{ Conn() *nats.Conn })
	if !ok {
		return nil
	}
	return accessor.Conn()
}

// GetJetStreamContext 从公共 broker 接口提取底层 JetStream context。
func GetJetStreamContext(instance broker.Broker) nats.JetStreamContext {
	accessor, ok := instance.(interface {
		JetStreamContext() nats.JetStreamContext
	})
	if !ok {
		return nil
	}
	return accessor.JetStreamContext()
}

// JetStreamMsgFromEvent 从事件中提取底层 NATS 消息。
func JetStreamMsgFromEvent(event broker.Event) (*nats.Msg, bool) {
	if event == nil {
		return nil, false
	}
	message, ok := event.RawMessage().(*nats.Msg)
	return message, ok
}

// normalizeContext 将 nil context 归一为 Background。
func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// applyMessageHeaders 将 broker 单值 header 写入 NATS 消息。
func applyMessageHeaders(message *nats.Msg, headers broker.Headers) {
	for key, value := range headers {
		message.Header.Set(key, value)
	}
}

// applyHeaders 将 NATS 多值 header 写入消息。
func applyHeaders(message *nats.Msg, headers map[string][]string) {
	for key, values := range headers {
		for _, value := range values {
			message.Header.Add(key, value)
		}
	}
}

// toBrokerHeaders 将 NATS 多值 header 转换为公共单值 header。
func toBrokerHeaders(headers nats.Header) broker.Headers {
	if len(headers) == 0 {
		return nil
	}
	converted := make(broker.Headers, len(headers))
	for key := range headers {
		converted[key] = headers.Get(key)
	}
	return converted
}
