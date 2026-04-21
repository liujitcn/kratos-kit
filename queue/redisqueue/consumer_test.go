package redisqueue

import (
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConsumerWorkDrainsBufferedMessagesAfterQueueClosed 验证 worker 在队列关闭后仍会继续处理已缓冲消息。
func TestConsumerWorkDrainsBufferedMessagesAfterQueueClosed(t *testing.T) {
	var handled atomic.Int32

	consumer := &Consumer{
		Errors: make(chan error, 3),
		consumers: map[string]registeredConsumer{
			"orders": {
				id: "0",
				fn: func(msg *Message) error {
					handled.Add(1)
					return stderrors.New("mock handler error")
				},
			},
		},
		queue: make(chan *Message, 3),
		wg:    &sync.WaitGroup{},
		options: &ConsumerOptions{
			GroupName: "test-group",
		},
	}

	consumer.queue <- &Message{ID: "1-0", Stream: "orders"}
	consumer.queue <- &Message{ID: "2-0", Stream: "orders"}
	consumer.queue <- &Message{ID: "3-0", Stream: "orders"}
	close(consumer.queue)

	consumer.wg.Add(1)
	go consumer.work()
	consumer.wg.Wait()

	if handled.Load() != 3 {
		t.Fatalf("expected 3 buffered messages to be handled, got %d", handled.Load())
	}
	if len(consumer.Errors) != 3 {
		t.Fatalf("expected 3 reported errors, got %d", len(consumer.Errors))
	}
}
