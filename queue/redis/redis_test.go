package redis

import (
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/queue/redisqueue"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestBuildProducerOptionsUsesApproximateTrimByDefault 验证未显式配置生产者时仍会启用默认近似裁剪。
func TestBuildProducerOptionsUsesApproximateTrimByDefault(t *testing.T) {
	redisOptions := &redisqueue.RedisOptions{Addrs: []string{"127.0.0.1:6379"}}

	options := buildProducerOptions(redisOptions, &conf.Data_Queue{})

	if options.RedisOptions != redisOptions {
		t.Fatal("expected redis options to be preserved")
	}
	if !options.ApproximateMaxLength {
		t.Fatal("expected approximate max length to default to true")
	}
}

// TestBuildProducerOptionsUsesConfiguredValues 验证生产者配置中的裁剪参数会被正确透传。
func TestBuildProducerOptionsUsesConfiguredValues(t *testing.T) {
	options := buildProducerOptions(nil, &conf.Data_Queue{
		Redis: &conf.Data_Queue_Redis{
			Producer: &conf.Data_Queue_Redis_Producer{
				StreamMaxLength:      256,
				ApproximateMaxLength: false,
			},
		},
	})

	if options.StreamMaxLength != 256 {
		t.Fatalf("expected stream max length 256, got %d", options.StreamMaxLength)
	}
	if options.ApproximateMaxLength {
		t.Fatal("expected approximate max length to remain disabled")
	}
}

// TestBuildConsumerOptionsAllowsPartialDurationConfig 验证消费者配置缺少部分时长字段时不会触发空指针。
func TestBuildConsumerOptionsAllowsPartialDurationConfig(t *testing.T) {
	redisOptions := &redisqueue.RedisOptions{Addrs: []string{"127.0.0.1:6379"}}

	options := buildConsumerOptions(redisOptions, &conf.Data_Queue{
		Redis: &conf.Data_Queue_Redis{
			Consumer: &conf.Data_Queue_Redis_Consumer{
				BlockingTimeout: durationpb.New(2 * time.Second),
				BufferSize:      8,
				Concurrency:     4,
			},
		},
	})

	if options.RedisOptions != redisOptions {
		t.Fatal("expected redis options to be preserved")
	}
	if options.VisibilityTimeout != 0 {
		t.Fatalf("expected zero visibility timeout, got %s", options.VisibilityTimeout)
	}
	if options.BlockingTimeout != 2*time.Second {
		t.Fatalf("expected blocking timeout 2s, got %s", options.BlockingTimeout)
	}
	if options.ReclaimInterval != 0 {
		t.Fatalf("expected zero reclaim interval, got %s", options.ReclaimInterval)
	}
	if options.BufferSize != 8 {
		t.Fatalf("expected buffer size 8, got %d", options.BufferSize)
	}
	if options.Concurrency != 4 {
		t.Fatalf("expected concurrency 4, got %d", options.Concurrency)
	}
}
