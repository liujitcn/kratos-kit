package mqtt

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/broker"
	"github.com/liujitcn/kratos-kit/broker/mqtt"
)

// Registration 描述一个 MQTT 主题订阅。
type Registration struct {
	Topic   string
	Handler broker.Handler
	Binder  broker.Binder
	Options []broker.SubscribeOption
}

// NewRegistration 创建 MQTT 订阅注册项。
func NewRegistration(topic string, handler broker.Handler, binder broker.Binder, options ...broker.SubscribeOption) Registration {
	return Registration{Topic: topic, Handler: handler, Binder: binder, Options: append([]broker.SubscribeOption(nil), options...)}
}

// Server 将 MQTT 客户端订阅接入 Kratos 应用生命周期。
type Server struct {
	mu      sync.Mutex
	broker  broker.Broker
	options []broker.Option
	started bool
	items   map[string]Registration
	subs    map[string]broker.Subscriber
}

// Stats 是 MQTT 服务运行统计快照。
type Stats = mqtt.Stats

var _ transport.Server = (*Server)(nil)

// ServerOption 配置 MQTT 订阅服务。
type ServerOption func(*Server)

// WithBrokerOptions 将底层 MQTT Broker 配置传入服务。
func WithBrokerOptions(options ...broker.Option) ServerOption {
	return func(server *Server) {
		server.options = append(server.options, options...)
	}
}

// WithAddress 设置 MQTT Broker 地址。
func WithAddress(addresses ...string) ServerOption {
	return WithBrokerOptions(broker.WithAddress(addresses...))
}

// WithCodec 设置 MQTT 消息编解码器。
func WithCodec(codec string) ServerOption {
	return WithBrokerOptions(broker.WithCodec(codec))
}

// WithAuth 设置 MQTT Broker 认证信息。
func WithAuth(username, password string) ServerOption {
	return WithBrokerOptions(mqtt.WithAuth(username, password))
}

// WithClientID 设置 MQTT 客户端 ID。
func WithClientID(clientID string) ServerOption {
	return WithBrokerOptions(mqtt.WithClientID(clientID))
}

// WithCleanSession 设置 MQTT Clean Session。
func WithCleanSession(clean bool) ServerOption {
	return WithBrokerOptions(mqtt.WithCleanSession(clean))
}

// WithKeepAlive 设置 MQTT 心跳周期。
func WithKeepAlive(keepAlive time.Duration) ServerOption {
	return WithBrokerOptions(mqtt.WithKeepAlive(keepAlive))
}

// WithAutoReconnect 设置 MQTT 是否自动重连。
func WithAutoReconnect(autoReconnect bool) ServerOption {
	return WithBrokerOptions(mqtt.WithAutoReconnect(autoReconnect))
}

// WithResumeSubs 设置 MQTT 重连后是否恢复订阅。
func WithResumeSubs(resumeSubs bool) ServerOption {
	return WithBrokerOptions(mqtt.WithResumeSubs(resumeSubs))
}

// WithOrderMatters 设置 MQTT 是否保持消息顺序。
func WithOrderMatters(orderMatters bool) ServerOption {
	return WithBrokerOptions(mqtt.WithOrderMatters(orderMatters))
}

// WithWriteTimeout 设置 MQTT 写操作超时时间。
func WithWriteTimeout(timeout time.Duration) ServerOption {
	return WithBrokerOptions(mqtt.WithWriteTimeout(timeout))
}

// WithConnectRetryInterval 设置 MQTT 连接重试间隔。
func WithConnectRetryInterval(interval time.Duration) ServerOption {
	return WithBrokerOptions(mqtt.WithConnectRetryInterval(interval))
}

// WithMaxReconnectInterval 设置 MQTT 最大重连间隔。
func WithMaxReconnectInterval(interval time.Duration) ServerOption {
	return WithBrokerOptions(mqtt.WithMaxReconnectInterval(interval))
}

// NewServer 创建 MQTT 订阅服务。
func NewServer(options ...ServerOption) *Server {
	server := &Server{items: make(map[string]Registration), subs: make(map[string]broker.Subscriber)}
	for _, option := range options {
		if option != nil {
			option(server)
		}
	}
	server.broker = mqtt.NewBroker(server.options...)
	return server
}

// NewServerWithBroker 使用外部 Broker 创建 MQTT 订阅服务。
func NewServerWithBroker(instance broker.Broker) *Server {
	return &Server{broker: instance, items: make(map[string]Registration), subs: make(map[string]broker.Subscriber)}
}

// Start 连接 MQTT Broker 并注册全部订阅。
func (s *Server) Start(_ context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if s.broker == nil {
		return errors.New("mqtt transport: broker is nil")
	}
	if err := s.broker.Init(); err != nil {
		return err
	}
	if err := s.broker.Connect(); err != nil {
		return err
	}
	for topic, registration := range s.items {
		subscriber, err := s.broker.Subscribe(topic, registration.Handler, registration.Binder, registration.Options...)
		if err != nil {
			unsubscribeErr := s.unsubscribeAll()
			disconnectErr := s.broker.Disconnect()
			return errors.Join(err, unsubscribeErr, disconnectErr)
		}
		s.subs[topic] = subscriber
	}
	s.started = true
	return nil
}

// Stop 取消全部订阅并断开 MQTT Broker。
func (s *Server) Stop(_ context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	err := s.unsubscribeAll()
	if disconnectErr := s.broker.Disconnect(); disconnectErr != nil && err == nil {
		err = disconnectErr
	}
	s.started = false
	return err
}

// Register 注册或替换一个 MQTT 主题订阅。
func (s *Server) Register(registration Registration) error {
	if s == nil {
		return errors.New("mqtt transport: server is nil")
	}
	if registration.Topic == "" || registration.Handler == nil {
		return errors.New("mqtt transport: topic and handler are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.subs[registration.Topic]; ok {
		if err := old.Unsubscribe(false); err != nil {
			return err
		}
		delete(s.subs, registration.Topic)
	}
	s.items[registration.Topic] = Registration{
		Topic:   registration.Topic,
		Handler: registration.Handler,
		Binder:  registration.Binder,
		Options: append([]broker.SubscribeOption(nil), registration.Options...),
	}
	if !s.started {
		return nil
	}
	subscriber, err := s.broker.Subscribe(registration.Topic, registration.Handler, registration.Binder, registration.Options...)
	if err != nil {
		return err
	}
	s.subs[registration.Topic] = subscriber
	return nil
}

// RegisterSubscriber 注册泛型 MQTT 订阅处理器。
func RegisterSubscriber[T any](server *Server, ctx context.Context, topic string, handler func(context.Context, string, broker.Headers, *T) error, options ...broker.SubscribeOption) error {
	if server == nil {
		return nil
	}
	if handler == nil {
		return errors.New("mqtt transport: subscriber handler is nil")
	}
	if ctx != nil {
		options = append(options, broker.WithSubscribeContext(ctx))
	}
	return server.Register(NewRegistration(topic, func(ctx context.Context, event broker.Event) error {
		if event == nil || event.Message() == nil || event.Message().Body == nil {
			return errors.New("mqtt transport: event body is nil")
		}
		body, ok := event.Message().Body.(*T)
		if !ok {
			return errors.New("mqtt transport: event body type mismatch")
		}
		return handler(ctx, event.Topic(), event.Message().Headers, body)
	}, func() any {
		var value T
		return &value
	}, options...))
}

// Broker 返回底层 MQTT Broker。
func (s *Server) Broker() broker.Broker {
	if s == nil {
		return nil
	}
	return s.broker
}

// Stats 返回 MQTT 服务连接、订阅和消息处理统计快照。
func (s *Server) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	s.mu.Lock()
	instance := s.broker
	registered := uint64(len(s.items))
	active := uint64(len(s.subs))
	s.mu.Unlock()
	provider, ok := instance.(mqtt.StatsProvider)
	if !ok {
		return Stats{RegisteredSubscriptions: registered, ActiveSubscriptions: active}
	}
	stats := provider.Stats()
	stats.RegisteredSubscriptions = registered
	stats.ActiveSubscriptions = active
	return stats
}

// unsubscribeAll 取消当前服务维护的全部订阅并清空运行时订阅快照。
func (s *Server) unsubscribeAll() error {
	var err error
	for topic, subscriber := range s.subs {
		if subscriberErr := subscriber.Unsubscribe(false); subscriberErr != nil && err == nil {
			err = subscriberErr
		}
		delete(s.subs, topic)
	}
	return err
}
