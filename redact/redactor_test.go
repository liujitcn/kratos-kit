package redact

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type emptyPolicyResolver struct{}

type fixedPolicyResolver struct {
	value string
}

// Resolve 返回当前测试实例独有的替换策略。
func (r fixedPolicyResolver) Resolve(context.Context, string) (FieldPolicy, bool) {
	return FieldPolicy{Mode: PolicyModeApplyRule, Transform: func(any) any { return r.value }}, true
}

// TestApplyWithIsolatesResolvers 验证上下文解析器相互隔离，显式参数优先且不影响无解析器调用。
func TestApplyWithIsolatesResolvers(t *testing.T) {
	for _, value := range []string{"first", "second"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			ctx := WithPolicyResolver(context.Background(), fixedPolicyResolver{value: value})
			message := wrapperspb.String("original")
			ApplyWith(ctx, nil, message)
			if message.Value != value {
				t.Fatalf("请求解析器未生效: %s", message.Value)
			}
			ApplyWith(ctx, fixedPolicyResolver{value: "explicit"}, message)
			if message.Value != "explicit" {
				t.Fatalf("显式解析器未覆盖上下文解析器: %s", message.Value)
			}
			if PolicyResolverFromContext(context.Background()) != nil || PolicyResolverFromContext(nil) != nil {
				t.Fatal("解析器泄露到其他请求")
			}
			if PolicyResolverFromContext(WithPolicyResolver(ctx, nil)) != nil {
				t.Fatal("显式空解析器未清除继承的请求策略")
			}
			message = wrapperspb.String("original")
			ApplyWith(context.Background(), nil, message)
			if message.Value != "original" {
				t.Fatalf("未注入解析器的请求受其他实例影响: %s", message.Value)
			}
		})
	}
}

// Resolve 表示测试解析器没有匹配到字段策略。
func (emptyPolicyResolver) Resolve(context.Context, string) (FieldPolicy, bool) {
	return FieldPolicy{}, false
}

// TestRuleTransforms 验证全部固定规则模板的实际转换结果。
func TestRuleTransforms(t *testing.T) {
	tests := []struct {
		name      string
		ruleType  string
		rule      string
		input     string
		want      string
		assertion func(*testing.T, string)
	}{
		{name: "MASK", ruleType: "MASK", rule: `{"mask":{"keep_first":3,"keep_last":4,"mask_char":"*"}}`, input: "13800138000", want: "138****8000"},
		{name: "EMAIL", ruleType: "EMAIL", rule: `{"email":{"keep_local_first":2,"mask_domain":false,"mask_char":"*"}}`, input: "alice@example.com", want: "al***@example.com"},
		{name: "REGEX", ruleType: "REGEX", rule: `{"regex":{"pattern":"(?s).+","replacement":"[REDACTED]"}}`, input: "token=secret-value", want: "[REDACTED]"},
		{name: "TRUNCATE", ruleType: "TRUNCATE", rule: `{"truncate":{"length":10,"suffix":"..."}}`, input: "RedactDemoUser", want: "RedactDemo..."},
		{name: "HASH", ruleType: "HASH", rule: `{"hash":{"algo":"SHA256"}}`, input: "sensitive-value", want: "423138f96007988c1dea6ef482c5f6b35e261a0e13df8dc34992ba8ba8a7f0b2"},
		{name: "UUID", ruleType: "UUID", rule: `{"uuid":{}}`, input: "sensitive-value", want: "77d7fbd7-9473-578b-b5fd-3e21ef7d31c6"},
		{name: "IP", ruleType: "IP", rule: `{"ip":{"keep_octets":2,"mask_char":"x"}}`, input: "192.168.10.25", want: "192.168.x.x"},
		{name: "URL", ruleType: "URL", rule: `{"url":{"mask_query":true,"mask_char":"*"}}`, input: "https://example.com/a?token=secret-value&user=redact_demo", assertion: func(t *testing.T, got string) {
			if strings.Contains(got, "secret-value") || strings.Contains(got, "redact_demo") {
				t.Fatalf("URL 查询参数未被掩码: %q", got)
			}
		}},
		{name: "FIXED_LENGTH", ruleType: "FIXED_LENGTH", rule: `{"fixed_length":{"char":"X"}}`, input: "secret", want: "XXXXXX"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := NewFieldPolicy(PolicyModeApplyRule, test.ruleType, test.rule)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := policy.Apply(test.input).(string)
			if !ok {
				t.Fatalf("规则结果不是字符串: %T", policy.Apply(test.input))
			}
			if test.assertion != nil {
				test.assertion(t, got)
				return
			}
			if got != test.want {
				t.Fatalf("规则结果错误: got=%q want=%q", got, test.want)
			}
		})
	}
}

// TestApplyDynamicKeepsUnconfiguredValue 验证启用解析器后未配置字段保持原值。
func TestApplyDynamicKeepsUnconfiguredValue(t *testing.T) {
	value := "phone=13800138000 email=alice@example.com"
	result, applied := ApplyDynamic(context.Background(), emptyPolicyResolver{}, "example.v1.Message.content", value)
	if applied || result != value {
		t.Fatalf("未配置字段不应执行隐式脱敏: value=%q applied=%v", result, applied)
	}
}

// TestApplyWithMessageMap 验证非空消息 Map 递归脱敏不会重复读取 Map 值描述符。
func TestApplyWithMessageMap(t *testing.T) {
	message := &structpb.Struct{Fields: map[string]*structpb.Value{
		"locale": structpb.NewStringValue("original"),
	}}
	ApplyWith(context.Background(), emptyPolicyResolver{}, message)
	if message.Fields["locale"].GetStringValue() != "original" {
		t.Fatal("无策略时不应改变 Map 内容")
	}
	ApplyWith(context.Background(), fixedPolicyResolver{value: "masked"}, message)
	if message.Fields["locale"].GetStringValue() != "masked" {
		t.Fatal("Map 消息字段未执行脱敏策略")
	}
}
