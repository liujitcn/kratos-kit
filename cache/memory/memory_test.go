package memory

import (
	"errors"
	"testing"

	"github.com/liujitcn/kratos-kit/cache/store"
)

// TestGetMissReturnsErrNotFound 验证未命中返回契约哨兵错误，调用方可以安全判定缓存未命中。
func TestGetMissReturnsErrNotFound(t *testing.T) {
	client, cleanup, err := NewMemory()
	if err != nil {
		t.Fatalf("创建内存缓存: %v", err)
	}
	defer cleanup()
	if _, err = client.Get("missing-key"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("未命中应返回 store.ErrNotFound: %v", err)
	}
	if _, err = client.GetDel("missing-key"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetDel 未命中应返回 store.ErrNotFound: %v", err)
	}
}
