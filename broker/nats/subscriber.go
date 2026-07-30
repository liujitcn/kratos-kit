package nats

import (
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/liujitcn/kratos-kit/broker"
)

type subscriberRemover interface {
	removeSubscriber(key string) bool
}

type subscriber struct {
	mu sync.RWMutex

	remover subscriberRemover
	key     string
	topic   string
	sub     *nats.Subscription
	options broker.SubscribeOptions
	closed  bool
}

// Options 返回订阅配置的快照。
func (s *subscriber) Options() broker.SubscribeOptions {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options
}

// Topic 返回订阅 subject。
func (s *subscriber) Topic() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.topic
}

// Unsubscribe 取消底层 NATS 订阅，并按需从 broker 管理器移除。
func (s *subscriber) Unsubscribe(removeFromManager bool) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	sub := s.sub
	s.mu.Unlock()

	var err error
	if sub != nil {
		err = sub.Unsubscribe()
	}
	if removeFromManager && s.remover != nil {
		s.remover.removeSubscriber(s.key)
	}
	return err
}

// IsClosed 判断订阅是否已经关闭。
func (s *subscriber) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

type publication struct {
	topic      string
	message    *broker.Message
	err        error
	ackEnabled bool
}

// Topic 返回消息 subject。
func (p *publication) Topic() string {
	return p.topic
}

// Message 返回解码后的 broker 消息。
func (p *publication) Message() *broker.Message {
	return p.message
}

// RawMessage 返回底层 NATS 消息。
func (p *publication) RawMessage() any {
	if p.message == nil {
		return nil
	}
	return p.message.Msg
}

// Ack 确认 JetStream 消息；Core NATS 消息不执行确认。
func (p *publication) Ack() error {
	if !p.ackEnabled || p.message == nil {
		return nil
	}
	message, ok := p.message.Msg.(*nats.Msg)
	if !ok {
		return nil
	}
	return message.Ack()
}

// Error 返回当前消息处理错误。
func (p *publication) Error() error {
	return p.err
}
