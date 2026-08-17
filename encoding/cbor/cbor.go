package cbor

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/go-kratos/kratos/v3/encoding"
)

// Name 是 CBOR codec 的注册名称。
const Name = "cbor"

type codec struct{}

// init 将 CBOR codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 将 Go 值编码为 CBOR。
func (codec) Marshal(value any) ([]byte, error) {
	return cbor.Marshal(value)
}

// Unmarshal 将 CBOR 数据解码到目标值。
func (codec) Unmarshal(data []byte, value any) error {
	return cbor.Unmarshal(data, value)
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
