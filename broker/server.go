package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v3/transport"
)

// Subscription 描述随 Kratos 应用启动的 broker 订阅。
type Subscription struct {
	// Topic 为订阅主题。
	Topic string
	// Handler 为消息处理方法。
	Handler Handler
	// Binder 创建消息体目标类型。
	Binder Binder
	// Options 为订阅配置。
	Options []SubscribeOption
}

// NewSubscription 创建随应用生命周期启动的 broker 订阅。
func NewSubscription(topic string, handler Handler, binder Binder, options ...SubscribeOption) Subscription {
	return Subscription{
		Topic:   topic,
		Handler: handler,
		Binder:  binder,
		Options: append([]SubscribeOption(nil), options...),
	}
}

// TransportServer 将任意 Broker 适配为 Kratos transport.Server。
type TransportServer struct {
	mu sync.Mutex

	broker        Broker
	subscriptions []Subscription
	started       bool
}

var _ transport.Server = (*TransportServer)(nil)

// NewTransportServer 创建通用 broker 生命周期适配器。
func NewTransportServer(instance Broker, subscriptions ...Subscription) *TransportServer {
	return &TransportServer{
		broker:        instance,
		subscriptions: append([]Subscription(nil), subscriptions...),
	}
}

// Start 初始化并连接 broker，然后建立预注册订阅。
func (s *TransportServer) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}
	if s.broker == nil {
		return errors.New("broker transport server: broker is nil")
	}

	var err error
	if err = s.broker.Init(); err != nil {
		return fmt.Errorf("初始化 broker: %w", err)
	}
	if err = s.broker.Connect(); err != nil {
		return fmt.Errorf("连接 broker: %w", err)
	}
	for _, subscription := range s.subscriptions {
		_, err = s.broker.Subscribe(
			subscription.Topic,
			subscription.Handler,
			subscription.Binder,
			subscription.Options...,
		)
		if err != nil {
			closeErr := s.broker.Disconnect()
			return errors.Join(fmt.Errorf("订阅 topic %q: %w", subscription.Topic, err), closeErr)
		}
	}
	s.started = true
	return nil
}

// Stop 断开 broker，并允许同一实例再次启动。
func (s *TransportServer) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}
	s.started = false
	var err error
	if err = s.broker.Disconnect(); err != nil {
		return fmt.Errorf("断开 broker: %w", err)
	}
	return nil
}
