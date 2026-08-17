package gob

import (
	"bytes"
	stdGob "encoding/gob"

	"github.com/go-kratos/kratos/v3/encoding"
)

// Name 是 gob codec 的注册名称。
const Name = "gob"

type codec struct{}

// init 将 gob codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 将 Go 值编码为 gob。
func (codec) Marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	err := stdGob.NewEncoder(&buffer).Encode(value)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Unmarshal 将 gob 数据解码到目标值。
func (codec) Unmarshal(data []byte, value any) error {
	return stdGob.NewDecoder(bytes.NewReader(data)).Decode(value)
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
