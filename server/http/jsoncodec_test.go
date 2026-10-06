package http

import (
	"testing"

	"github.com/go-kratos/kratos/v3/encoding"
)

type jsSafeCodecStruct struct {
	Small int64  `json:"small"`
	Big   int64  `json:"big"`
	UBig  uint64 `json:"u_big"`
}

func TestJSSafeJSONCodecRegistered(t *testing.T) {
	codec := encoding.GetCodec("json")
	if codec == nil {
		t.Fatalf("json codec 未注册")
	}
	if codec.Name() != "json" {
		t.Errorf("codec name = %s, 期望 json", codec.Name())
	}
}

func TestJSSafeJSONCodecMarshal(t *testing.T) {
	codec := encoding.GetCodec("json")
	data, err := codec.Marshal(&jsSafeCodecStruct{Small: 42, Big: 57309943358867204, UBig: 18446744073709551615})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	text := string(data)
	if want := `{"small":42,"big":"57309943358867204","u_big":"18446744073709551615"}`; text != want {
		t.Errorf("marshal 输出 = %s, 期望 %s", text, want)
	}
}

func TestJSSafeJSONCodecUnmarshal(t *testing.T) {
	codec := encoding.GetCodec("json")
	var value jsSafeCodecStruct
	if err := codec.Unmarshal([]byte(`{"small":"42","big":"57309943358867204","u_big":18446744073709551615}`), &value); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if value.Small != 42 || value.Big != 57309943358867204 || value.UBig != 18446744073709551615 {
		t.Errorf("unmarshal 结果 = %+v", value)
	}
}
