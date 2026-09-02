package runtime

import (
	"context"
	"testing"
)

type testKey struct{}

func (testKey) Derive(_ context.Context, _ string) ([]byte, error) {
	return []byte("key"), nil
}

func TestKey(t *testing.T) {
	runtime := NewRuntime()
	value := testKey{}
	runtime.SetKey(value)
	if got := runtime.GetKey(); got == nil {
		t.Fatal("GetKey returned nil")
	}
}
