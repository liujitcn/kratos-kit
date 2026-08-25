package response

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"net/http"

	"github.com/go-kratos/kratos/v3/encoding"
	kratosJSON "github.com/go-kratos/kratos/v3/encoding/json"
	kratosHttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/proto"
)

// ProtoJSONEncoder 编码 HTTP 响应。
// 仅当客户端协商结果为 JSON 且响应值为 protobuf 消息时，使用自定义 JSON 编码以数字形式输出 64 位整数。
// 其他场景保持 Kratos 默认 codec 行为，避免破坏内容协商和非 protobuf 响应。
func ProtoJSONEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if v == nil {
		return nil
	}
	if rd, ok := v.(kratosHttp.Redirector); ok {
		url, code := rd.Redirect()
		http.Redirect(w, r, url, code)
		return nil
	}

	codec, _ := kratosHttp.CodecForRequest(r, "Accept")
	data, err := marshal(codec, v)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", contentType(codec.Name()))
	_, err = w.Write(data)
	return err
}

// marshal 根据协商后的 codec 编码响应。
// JSON 与 protobuf 组合场景使用自定义编码器，其余场景回退到 Kratos codec，保证兼容性。
func marshal(codec encoding.Codec, v interface{}) ([]byte, error) {
	if codec.Name() == kratosJSON.Name {
		if message, ok := v.(proto.Message); ok {
			// 保留 encoding/json v1 的兼容语义，同时使用 Go 1.27 的 json/v2 实现。
			return jsonv2.Marshal(message, json.DefaultOptionsV1())
		}
	}

	return codec.Marshal(v)
}

const baseContentType = "application"

// contentType 返回带有 application 前缀的 Content-Type。
func contentType(subtype string) string {
	return baseContentType + "/" + subtype
}
