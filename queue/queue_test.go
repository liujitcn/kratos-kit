package queue

import "testing"

// TestNewQueueCleanupWithoutRun 验证内存队列在未运行时执行清理不会触发 panic。
func TestNewQueueCleanupWithoutRun(t *testing.T) {
	_, cleanup, err := NewQueue(nil, nil)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup is nil")
	}

	cleanup()
}
