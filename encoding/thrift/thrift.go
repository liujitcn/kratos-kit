// Package thrift 为 Thrift 生成类型提供 Kratos 二进制 codec。
package thrift

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kratos/kratos/v3/encoding"
)

// Name 是 Thrift codec 的注册名称。
const Name = "thrift"

type codec struct{}

// init 将 Thrift codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 使用 Thrift 二进制协议编码生成类型。
func (codec) Marshal(value any) ([]byte, error) {
	message, ok := value.(thrift.TStruct)
	if !ok {
		return nil, fmt.Errorf("thrift: value %T does not implement thrift.TStruct", value)
	}
	return thrift.NewTSerializer().Write(context.Background(), message)
}

// Unmarshal 使用 Thrift 二进制协议解码到生成类型。
func (codec) Unmarshal(data []byte, value any) error {
	message, ok := value.(thrift.TStruct)
	if !ok {
		return fmt.Errorf("thrift: target %T does not implement thrift.TStruct", value)
	}
	return thrift.NewTDeserializer().Read(context.Background(), message, data)
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
