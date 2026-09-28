package cache

import "testing"

// TestCacheTokenBucketCompatibility 验证便捷令牌桶接口复用统一限流实现。
func TestCacheTokenBucketCompatibility(t *testing.T) {
	cacheStore, cleanup, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(cleanup)

	request := []TokenBucketRequest{{Key: "compatibility", TokensPerSecond: 0.001, Burst: 1}}
	allowed, _, err := cacheStore.TakeTokenBuckets(request)
	if err != nil || !allowed {
		t.Fatalf("first TakeTokenBuckets() allowed = %t, error = %v", allowed, err)
	}
	allowed, retryAfter, err := cacheStore.TakeTokenBuckets(request)
	if err != nil || allowed || retryAfter <= 0 {
		t.Fatalf("second TakeTokenBuckets() allowed = %t, retryAfter = %s, error = %v", allowed, retryAfter, err)
	}
}

// TestCacheRevision 验证共享版本的初始值、递增和无效值处理。
func TestCacheRevision(t *testing.T) {
	cacheStore, cleanup, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(cleanup)

	revision, usable := ReadRevision(cacheStore, "revision")
	if !usable || revision != "0" {
		t.Fatalf("ReadRevision() = (%q, %t), want (\"0\", true)", revision, usable)
	}
	if err = IncrementRevision(cacheStore, "revision"); err != nil {
		t.Fatalf("IncrementRevision() error = %v", err)
	}
	revision, usable = ReadRevision(cacheStore, "revision")
	if !usable || revision != "1" {
		t.Fatalf("ReadRevision() = (%q, %t), want (\"1\", true)", revision, usable)
	}
	if err = cacheStore.Set("revision", "invalid", 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, usable = ReadRevision(cacheStore, "revision"); usable {
		t.Fatal("ReadRevision() accepted a non-numeric revision")
	}
}
