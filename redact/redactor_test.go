package redact

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

type testRedactor struct {
	value string
}

type testPolicyResolver struct {
	policy FieldPolicy
}

func (r testPolicyResolver) Resolve(_ context.Context, fieldRef string) (FieldPolicy, bool) {
	return r.policy, fieldRef == "google.protobuf.StringValue.value"
}

func (r *testRedactor) Redact() {
	r.value = Mask(r.value, 3, 4, "*")
}

// TestApply 验证 Apply 只对实现 Redactor 的值执行脱敏。
func TestApply(t *testing.T) {
	value := &testRedactor{value: "13812345678"}
	Apply(value)
	if value.value != "138****5678" {
		t.Fatalf("unexpected masked value: %s", value.value)
	}

	Apply(struct{}{})
}

// TestStringRedactors 验证字符串脱敏规则的运行时实现。
func TestStringRedactors(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "regex", got: Regex("13812345678", `^(\d{3})\d{4}(\d{4})$`, `${1}****${2}`), want: "138****5678"},
		{name: "mask", got: Mask("13812345678", 3, 4, "*"), want: "138****5678"},
		{name: "email", got: Email("alice@example.com", 2, false, "*"), want: "al***@example.com"},
		{name: "truncate", got: Truncate("Alexander", 1, "**"), want: "A**"},
		{name: "fixed_length", got: FixedLength("1234", "#"), want: "####"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("unexpected value: got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestNewFieldPolicy 验证数据库规则 JSON 可以构造成运行时策略。
func TestNewFieldPolicy(t *testing.T) {
	policy, err := NewFieldPolicy(PolicyModeApplyRule, "MASK", `{"mask":{"keep_first":3,"keep_last":4,"mask_char":"*"}}`)
	if err != nil {
		t.Fatalf("NewFieldPolicy returned error: %v", err)
	}
	if got := policy.Apply("13812345678"); got != "138****5678" {
		t.Fatalf("unexpected policy value: %v", got)
	}
}

// TestApplyWith 验证运行时策略可以通过 Proto 反射递归修改消息字段。
func TestApplyWith(t *testing.T) {
	value := wrapperspb.String("13812345678")
	policy, err := NewFieldPolicy(PolicyModeApplyRule, "MASK", `{"mask":{"keep_first":3,"keep_last":4}}`)
	if err != nil {
		t.Fatalf("NewFieldPolicy returned error: %v", err)
	}
	ApplyWith(context.Background(), testPolicyResolver{policy: policy}, value)
	if value.Value != "138****5678" {
		t.Fatalf("unexpected dynamic value: %s", value.Value)
	}
}

// TestCondition 验证环境变量条件判断。
func TestCondition(t *testing.T) {
	t.Setenv("REDACT_TEST_ENV", "production")
	if !Condition("REDACT_TEST_ENV", "production") {
		t.Fatal("expected condition to match")
	}
	if Condition("REDACT_TEST_ENV", "development") {
		t.Fatal("expected condition not to match")
	}
	if Condition("REDACT_MISSING_ENV", "") {
		t.Fatal("expected unset condition not to match")
	}
}

// TestCustomRedactor 验证自定义脱敏函数的注册和调用。
func TestCustomRedactor(t *testing.T) {
	RegisterCustomRedactor("test", func(value string) string {
		return "[" + value + "]"
	})
	if got := ApplyCustomRedactor("test", "secret"); got != "[secret]" {
		t.Fatalf("unexpected custom value: %s", got)
	}
	if got := ApplyCustomRedactor("missing", "secret"); got != "secret" {
		t.Fatalf("unexpected missing custom value: %s", got)
	}
}
