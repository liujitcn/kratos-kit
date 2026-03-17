package rpc

import (
	"net/http"

	"github.com/go-kratos/kratos/v2/encoding"
	kratosJSON "github.com/go-kratos/kratos/v2/encoding/json"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

var protoJSONResponseConfig = jsoniter.Config{
	EscapeHTML:             true,
	SortMapKeys:            true,
	UseNumber:              true,
	ValidateJsonRawMessage: true,
}.Froze()

// protoJSONResponseEncoder 编码 HTTP 响应。
// 仅当客户端协商结果为 JSON 且响应值为 protobuf 消息时，使用自定义 JSON 编码以数字形式输出 64 位整数。
// 其他场景保持 kratos 默认 codec 行为，避免破坏内容协商和非 protobuf 响应。
func protoJSONResponseEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if v == nil {
		return nil
	}
	if rd, ok := v.(kratosHttp.Redirector); ok {
		url, code := rd.Redirect()
		http.Redirect(w, r, url, code)
		return nil
	}

	codec, _ := kratosHttp.CodecForRequest(r, "Accept")
	data, err := marshalResponse(codec, v)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", contentType(codec.Name()))
	_, err = w.Write(data)
	return err
}

// marshalResponse 根据协商后的 codec 编码响应。
// JSON + protobuf 场景使用自定义编码器，其余场景回退到 kratos codec，保证兼容性。
func marshalResponse(codec encoding.Codec, v interface{}) ([]byte, error) {
	if codec.Name() == kratosJSON.Name {
		if m, ok := v.(proto.Message); ok {
			return protoJSONResponseConfig.Marshal(m)
		}
	}

	return codec.Marshal(v)
}

const (
	baseContentType = "application"
)

// contentType 返回带有 application 前缀的 Content-Type。
func contentType(subtype string) string {
	return baseContentType + "/" + subtype
}
