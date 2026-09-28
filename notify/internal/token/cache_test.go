package token

import (
	"context"
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestCacheSharesTokenByChannelInstanceAndCredentials(t *testing.T) {
	shared, cleanup, err := cache.NewCache(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	first, err := New(shared, notify.FeishuApp, "finance", "app-id", "app-secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(shared, notify.FeishuApp, "finance", "app-id", "app-secret")
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	refresh := func(context.Context) (string, time.Duration, error) {
		fetches++
		return "tenant-token", time.Hour, nil
	}
	for _, tokenCache := range []*Cache{first, second} {
		value, getErr := tokenCache.Get(context.Background(), refresh)
		if getErr != nil || value != "tenant-token" {
			t.Fatalf("Get() = %q, %v", value, getErr)
		}
	}
	if fetches != 1 {
		t.Fatalf("expected the shared cache to avoid a second fetch, got %d", fetches)
	}
	otherApp, err := New(shared, notify.FeishuApp, "finance", "other-app", "other-secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = otherApp.Get(context.Background(), refresh)
	if err != nil {
		t.Fatal(err)
	}
	if fetches != 2 {
		t.Fatalf("different app credentials reused a token, fetch count %d", fetches)
	}
}
