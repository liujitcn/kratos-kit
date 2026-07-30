// Package flatbuffers 为生成的 FlatBuffers 类型提供 Kratos codec 桥接。
package flatbuffers

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/encoding"
	flatbuffers "github.com/google/flatbuffers/go"
)

// Name 是 FlatBuffers codec 的注册名称。
const Name = "flatbuffers"

// Marshaler 定义生成类型序列化为完整 FlatBuffer 的能力。
type Marshaler interface {
	PackFlatBuffer() ([]byte, error)
}

type codec struct{}

// init 将 FlatBuffers codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 调用生成类型提供的 PackFlatBuffer。
func (codec) Marshal(value any) ([]byte, error) {
	marshaler, ok := value.(Marshaler)
	if !ok {
		return nil, fmt.Errorf("flatbuffers: value %T does not implement Marshaler", value)
	}
	return marshaler.PackFlatBuffer()
}

// Unmarshal 初始化实现 flatbuffers.FlatBuffer 的生成目标类型。
func (codec) Unmarshal(data []byte, value any) error {
	target, ok := value.(flatbuffers.FlatBuffer)
	if !ok {
		return fmt.Errorf("flatbuffers: target %T does not implement flatbuffers.FlatBuffer", value)
	}
	if len(data) < flatbuffers.SizeUOffsetT {
		return fmt.Errorf("flatbuffers: buffer length %d is less than root offset size %d", len(data), flatbuffers.SizeUOffsetT)
	}
	rootOffset := flatbuffers.GetUOffsetT(data)
	if rootOffset > flatbuffers.UOffsetT(len(data)-flatbuffers.SizeUOffsetT) {
		return fmt.Errorf("flatbuffers: root offset %d is outside buffer length %d", rootOffset, len(data))
	}
	target.Init(data, rootOffset)
	return nil
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
