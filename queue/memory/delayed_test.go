package memory

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/queue/data"
)

func TestDelayedMessage(t *testing.T) {
	queue := NewMemory(8)
	defer queue.Shutdown()
	received := make(chan data.Message, 1)
	queue.Register("test", func(message data.Message) error {
		received <- message
		return nil
	})
	err := queue.Schedule("test", data.Message{ID: "one", Values: map[string]interface{}{"data": "value"}}, time.Now().Add(20*time.Millisecond))
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	select {
	case message := <-received:
		if message.ID != "one" {
			t.Fatalf("message ID = %q, want one", message.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("delayed message was not delivered")
	}
}

func TestCancelDelayedMessage(t *testing.T) {
	queue := NewMemory(8)
	defer queue.Shutdown()
	var count atomic.Int32
	queue.Register("test", func(data.Message) error {
		count.Add(1)
		return nil
	})
	err := queue.Schedule("test", data.Message{ID: "cancelled"}, time.Now().Add(30*time.Millisecond))
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if err = queue.Cancel("test", "cancelled"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if count.Load() != 0 {
		t.Fatalf("delivery count = %d, want 0", count.Load())
	}
}

func TestRescheduleDelayedMessage(t *testing.T) {
	queue := NewMemory(8)
	defer queue.Shutdown()
	received := make(chan time.Time, 2)
	queue.Register("test", func(data.Message) error {
		received <- time.Now()
		return nil
	})
	message := data.Message{ID: "rescheduled"}
	if err := queue.Schedule("test", message, time.Now().Add(20*time.Millisecond)); err != nil {
		t.Fatalf("first Schedule() error = %v", err)
	}
	wantAfter := time.Now().Add(80 * time.Millisecond)
	if err := queue.Schedule("test", message, wantAfter); err != nil {
		t.Fatalf("second Schedule() error = %v", err)
	}
	select {
	case deliveredAt := <-received:
		if deliveredAt.Before(wantAfter.Add(-10 * time.Millisecond)) {
			t.Fatalf("message delivered at %v, want after %v", deliveredAt, wantAfter)
		}
	case <-time.After(time.Second):
		t.Fatal("rescheduled message was not delivered")
	}
	select {
	case <-received:
		t.Fatal("rescheduled message was delivered more than once")
	case <-time.After(80 * time.Millisecond):
	}
}
