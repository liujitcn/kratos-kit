package response

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

type jsSafeStruct struct {
	Small  int64            `json:"small"`
	Big    int64            `json:"big"`
	USmall uint64           `json:"u_small"`
	UBig   uint64           `json:"u_big"`
	Ratio  float64          `json:"ratio"`
	Cost   time.Duration    `json:"cost"`
	Tags   []int64          `json:"tags"`
	Counts map[string]int64 `json:"counts"`
}

// namedInt64 用于确认命名 int64 类型不受类型级序列化规则影响。
type namedInt64 int64

type jsSafeNamedStruct struct {
	Value namedInt64 `json:"value"`
}

func TestMarshalJSSafe(t *testing.T) {
	data, err := MarshalJSSafe(&jsSafeStruct{
		Small:  42,
		Big:    57309943358867204,
		USmall: 874480550678528,
		UBig:   9223372036854775808,
		Ratio:  1.5,
		Cost:   1500,
		Tags:   []int64{1, 9007199254740992},
		Counts: map[string]int64{"big": 57309943358867204},
	})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"small":42`,
		`"big":"57309943358867204"`,
		`"u_small":874480550678528`,
		`"u_big":"9223372036854775808"`,
		`"ratio":1.5`,
		`"cost":1500`,
		`"tags":[1,"9007199254740992"]`,
		`"counts":{"big":"57309943358867204"}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("输出缺少 %s: %s", want, text)
		}
	}
}

func TestMarshalJSSafeNamedInt64(t *testing.T) {
	data, err := MarshalJSSafe(&jsSafeNamedStruct{Value: namedInt64(57309943358867204)})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	// 命名类型保持既有数字行为，避免影响 time.Duration 等场景。
	if got, want := string(data), `{"value":57309943358867204}`; got != want {
		t.Errorf("命名类型输出 = %s, 期望 %s", got, want)
	}
}

func TestMarshalJSSafeProtoMessage(t *testing.T) {
	data, err := MarshalJSSafe(durationpb.New(time.Second))
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	// 小值时间戳保持数字，零值字段按 v1 omitempty 语义省略，与既有响应形态一致。
	if got, want := string(data), `{"seconds":1}`; got != want {
		t.Errorf("proto 输出 = %s, 期望 %s", got, want)
	}
}

func TestUnmarshalJSSafe(t *testing.T) {
	var value jsSafeStruct
	data := `{"small":42,"big":"57309943358867204","u_small":874480550678528,"u_big":"9223372036854775808","big_null":null}`
	if err := UnmarshalJSSafe([]byte(data), &value); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if value.Small != 42 {
		t.Errorf("Small = %d, 期望 42", value.Small)
	}
	if value.Big != 57309943358867204 {
		t.Errorf("Big = %d, 期望 57309943358867204", value.Big)
	}
	if value.USmall != 874480550678528 {
		t.Errorf("USmall = %d, 期望 874480550678528", value.USmall)
	}
	if value.UBig != 9223372036854775808 {
		t.Errorf("UBig = %d, 期望 9223372036854775808", value.UBig)
	}
}

func TestUnmarshalJSSafeNumberAndNull(t *testing.T) {
	var value struct {
		Big   int64  `json:"big"`
		Empty *int64 `json:"empty"`
	}
	if err := UnmarshalJSSafe([]byte(`{"big":57309943358867204,"empty":null}`), &value); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if value.Big != 57309943358867204 {
		t.Errorf("Big = %d, 期望 57309943358867204", value.Big)
	}
	if value.Empty != nil {
		t.Errorf("Empty 应保持 nil")
	}
}

func TestUnmarshalJSSafeInvalidString(t *testing.T) {
	var value struct {
		Big int64 `json:"big"`
	}
	if err := UnmarshalJSSafe([]byte(`{"big":"not-a-number"}`), &value); err == nil {
		t.Errorf("非法字符串应返回错误")
	}
}
