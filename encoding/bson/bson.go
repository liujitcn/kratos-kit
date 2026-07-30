// Package bson 将 MongoDB BSON 编解码器注册为 Kratos codec。
package bson

import (
	"github.com/go-kratos/kratos/v3/encoding"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Name 是 BSON codec 的注册名称。
const Name = "bson"

type codec struct{}

// init 将 BSON codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 将 Go 值编码为 BSON。
func (codec) Marshal(value any) ([]byte, error) {
	return bson.Marshal(value)
}

// Unmarshal 将 BSON 数据解码到目标值。
func (codec) Unmarshal(data []byte, value any) error {
	return bson.Unmarshal(data, value)
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
