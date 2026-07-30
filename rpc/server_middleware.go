package rpc

import (
	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v3/middleware"
	kratosValidate "github.com/go-kratos/kratos/v3/middleware/validate"
	"google.golang.org/protobuf/proto"
)

// protoValidateMiddleware 使用 Kratos 校验中间件执行 ProtoValidate 和旧式 Validate 校验。
func protoValidateMiddleware() middleware.Middleware {
	return kratosValidate.Validator(func(value any) error {
		message, ok := value.(proto.Message)
		if !ok {
			return nil
		}
		return protovalidate.Validate(message)
	})
}
