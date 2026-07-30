// Package avro 提供必须绑定 schema 的 Kratos Avro codec。
package avro

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/linkedin/goavro/v2"
)

// Name 是 Avro codec 的名称。
const Name = "avro"

type codec struct {
	avroCodec *goavro.Codec
}

// NewCodec 根据 JSON 编码的 Avro schema 创建 codec。
//
// 返回的实例不会自动注册，因为不同消息通常使用不同 schema。调用方只应在
// 整个进程共享同一个 schema 时将其注册到 Kratos。
func NewCodec(schema string) (encoding.Codec, error) {
	avroCodec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("avro: invalid schema: %w", err)
	}
	return codec{avroCodec: avroCodec}, nil
}

// Marshal 将与 schema 匹配的 Go 原生值编码为 Avro 二进制。
func (c codec) Marshal(value any) ([]byte, error) {
	data, err := c.avroCodec.BinaryFromNative(nil, value)
	if err != nil {
		return nil, fmt.Errorf("avro: marshal: %w", err)
	}
	return data, nil
}

// Unmarshal 将 Avro 二进制解码到 *any 或 *map[string]any。
func (c codec) Unmarshal(data []byte, value any) error {
	native, remaining, err := c.avroCodec.NativeFromBinary(data)
	if err != nil {
		return fmt.Errorf("avro: unmarshal: %w", err)
	}
	if len(remaining) != 0 {
		return fmt.Errorf("avro: unmarshal left %d trailing bytes", len(remaining))
	}

	switch target := value.(type) {
	case *any:
		*target = native
		return nil
	case *map[string]any:
		record, ok := native.(map[string]any)
		if !ok {
			return fmt.Errorf("avro: decoded value is %T, want map[string]any", native)
		}
		*target = record
		return nil
	default:
		return fmt.Errorf("avro: unsupported unmarshal target %T", value)
	}
}

// Name 返回 codec 名称。
func (codec) Name() string {
	return Name
}
