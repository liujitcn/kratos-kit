package redisqueue

import (
	"context"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"
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

type stubStreamAckDeleter struct {
	ackCalls int
	delCalls int
	ackErr   error
	delErr   error
}

func (s *stubStreamAckDeleter) XAck(ctx context.Context, stream string, group string, ids ...string) *redis.IntCmd {
	s.ackCalls++
	cmd := redis.NewIntCmd(ctx)
	if s.ackErr != nil {
		cmd.SetErr(s.ackErr)
		return cmd
	}

	cmd.SetVal(1)
	return cmd
}

func (s *stubStreamAckDeleter) XDel(ctx context.Context, stream string, ids ...string) *redis.IntCmd {
	s.delCalls++
	cmd := redis.NewIntCmd(ctx)
	if s.delErr != nil {
		cmd.SetErr(s.delErr)
		return cmd
	}

	cmd.SetVal(1)
	return cmd
}

// TestAcknowledgeMessageDeletesAfterAck 验证确认成功后会删除消息。
func TestAcknowledgeMessageDeletesAfterAck(t *testing.T) {
	client := &stubStreamAckDeleter{}

	err := acknowledgeMessage(context.Background(), client, "test-group", &Message{
		ID:     "1-0",
		Stream: "orders",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if client.ackCalls != 1 {
		t.Fatalf("expected ack to be called once, got %d", client.ackCalls)
	}
	if client.delCalls != 1 {
		t.Fatalf("expected delete to be called once, got %d", client.delCalls)
	}
}

// TestAcknowledgeMessageStopsWhenAckFails 验证确认失败时不会继续执行删除。
func TestAcknowledgeMessageStopsWhenAckFails(t *testing.T) {
	client := &stubStreamAckDeleter{ackErr: stderrors.New("ack failed")}

	err := acknowledgeMessage(context.Background(), client, "test-group", &Message{
		ID:     "1-0",
		Stream: "orders",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if client.ackCalls != 1 {
		t.Fatalf("expected ack to be called once, got %d", client.ackCalls)
	}
	if client.delCalls != 0 {
		t.Fatalf("expected delete to be skipped after ack failure, got %d calls", client.delCalls)
	}
}
