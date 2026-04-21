package redisqueue

import "testing"

// TestNormalizeProducerOptionsUsesDefaultsWhenNil 验证空生产者配置会回填默认值。
func TestNormalizeProducerOptionsUsesDefaultsWhenNil(t *testing.T) {
	options := normalizeProducerOptions(nil)

	if options.StreamMaxLength != defaultProducerOptions.StreamMaxLength {
		t.Fatalf("expected default stream max length %d, got %d", defaultProducerOptions.StreamMaxLength, options.StreamMaxLength)
	}
	if options.ApproximateMaxLength != defaultProducerOptions.ApproximateMaxLength {
		t.Fatalf("expected default approximate max length %t, got %t", defaultProducerOptions.ApproximateMaxLength, options.ApproximateMaxLength)
	}
}

// TestNormalizeProducerOptionsPreservesExplicitApproximateFlag 验证显式关闭近似裁剪时不会被默认值覆盖。
func TestNormalizeProducerOptionsPreservesExplicitApproximateFlag(t *testing.T) {
	options := normalizeProducerOptions(&ProducerOptions{
		ApproximateMaxLength: false,
	})

	if options.StreamMaxLength != defaultProducerOptions.StreamMaxLength {
		t.Fatalf("expected default stream max length %d, got %d", defaultProducerOptions.StreamMaxLength, options.StreamMaxLength)
	}
	if options.ApproximateMaxLength {
		t.Fatal("expected approximate max length to remain disabled")
	}
}

// TestBuildXAddArgsCarriesTrimOptions 验证 XADD 参数会携带长度裁剪配置。
func TestBuildXAddArgsCarriesTrimOptions(t *testing.T) {
	producer := &Producer{
		options: &ProducerOptions{
			StreamMaxLength:      128,
			ApproximateMaxLength: true,
		},
	}
	message := &Message{
		ID:     "1-0",
		Stream: "orders",
		Values: map[string]interface{}{"orderId": "1001"},
	}

	args := producer.buildXAddArgs(message)

	if args.ID != message.ID {
		t.Fatalf("expected id %q, got %q", message.ID, args.ID)
	}
	if args.Stream != message.Stream {
		t.Fatalf("expected stream %q, got %q", message.Stream, args.Stream)
	}
	values, ok := args.Values.(map[string]interface{})
	if !ok {
		t.Fatalf("expected values to remain a map, got %T", args.Values)
	}
	if values["orderId"] != "1001" {
		t.Fatalf("expected values to be preserved, got %#v", values)
	}
	if args.MaxLen != producer.options.StreamMaxLength {
		t.Fatalf("expected max len %d, got %d", producer.options.StreamMaxLength, args.MaxLen)
	}
	if args.Approx != producer.options.ApproximateMaxLength {
		t.Fatalf("expected approx %t, got %t", producer.options.ApproximateMaxLength, args.Approx)
	}
}
