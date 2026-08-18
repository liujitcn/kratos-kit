package memory

import (
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/queue/data"
)

// TestShutdownStopsConsumer 验证内存队列停止时消费者和 Run 都能退出。
func TestShutdownStopsConsumer(t *testing.T) {
	queue := NewMemory(1)
	consumed := make(chan struct{})
	queue.Register("test", func(data.Message) error {
		close(consumed)
		return nil
	})

	runDone := make(chan struct{})
	go func() {
		queue.Run()
		close(runDone)
	}()
	if err := queue.Append("test", data.Message{}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	select {
	case <-consumed:
	case <-time.After(time.Second):
		t.Fatal("consumer did not receive message")
	}

	queue.Shutdown()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
	if err := queue.Append("test", data.Message{}); err == nil {
		t.Fatal("Append() after Shutdown() returned nil")
	}
}

func TestShutdownFromConsumerDoesNotDeadlock(t *testing.T) {
	queue := NewMemory(1)
	consumerDone := make(chan struct{})
	queue.Register("test", func(data.Message) error {
		queue.Shutdown()
		close(consumerDone)
		return nil
	})

	runDone := make(chan struct{})
	go func() {
		queue.Run()
		close(runDone)
	}()
	if err := queue.Append("test", data.Message{}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	select {
	case <-consumerDone:
	case <-time.After(time.Second):
		t.Fatal("consumer did not finish")
	}
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}
