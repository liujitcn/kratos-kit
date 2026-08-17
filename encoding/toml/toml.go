package toml

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/kratos/v3/encoding"
)

// Name 是 TOML codec 的注册名称。
const Name = "toml"

type codec struct{}

// init 将 TOML codec 注册到 Kratos。
func init() {
	encoding.RegisterCodec(codec{})
}

// Marshal 将 Go 值编码为 TOML。
func (codec) Marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	err := toml.NewEncoder(&buffer).Encode(value)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Unmarshal 将 TOML 数据解码到目标值。
func (codec) Unmarshal(data []byte, value any) error {
	return toml.Unmarshal(data, value)
}

// Name 返回 codec 注册名称。
func (codec) Name() string {
	return Name
}
