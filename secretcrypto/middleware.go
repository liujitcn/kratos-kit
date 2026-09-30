package secretcrypto

import (
	"context"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v3/middleware"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const googleWellKnownPrefix = "google.protobuf."

// secretTargetMemo 缓存消息类型是否携带目标密文字段的判定结果。
var secretTargetMemo sync.Map

// Middleware 返回统一解密敏感字段密文的 Kratos 中间件。
// 请求消息中全部目标类型字段在进入业务前解密回填明文，处理完成后立即清除；
// 中间件应挂在调用链最内侧，保证日志中间件只见密文。
func (s *Service) Middleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if msg, ok := req.(proto.Message); ok && msg != nil {
				if err := s.decryptMessage(msg); err != nil {
					return nil, err
				}
			}
			resp, err := handler(ctx, req)
			if msg, ok := req.(proto.Message); ok && msg != nil {
				s.clearMessage(msg)
			}
			return resp, err
		}
	}
}

// decryptMessage 解密请求消息树中的全部目标密文字段并回填明文。
func (s *Service) decryptMessage(req proto.Message) error {
	targets := collectSecretMessages(req.ProtoReflect(), protoreflect.FullName(s.messageType))
	if len(targets) == 0 {
		return nil
	}
	ciphers := make([]*Cipher, 0, len(targets))
	for _, item := range targets {
		ciphers = append(ciphers, cipherFromMessage(item, s.textField))
	}
	if err := s.Decrypt(ciphers...); err != nil {
		return err
	}
	for i, item := range ciphers {
		setMessageString(targets[i], s.textField, item.Text)
	}
	return nil
}

// clearMessage 清除消息树中全部目标密文字段的明文。
func (s *Service) clearMessage(req proto.Message) {
	targets := collectSecretMessages(req.ProtoReflect(), protoreflect.FullName(s.messageType))
	for _, item := range targets {
		setMessageString(item, s.textField, "")
	}
}

// cipherFromMessage 从密文消息中读取传输字段构建解密载体。
func cipherFromMessage(m protoreflect.Message, textFieldName string) *Cipher {
	return &Cipher{
		KeyID:        messageString(m, "key_id"),
		Nonce:        messageString(m, "nonce"),
		Algorithm:    messageString(m, "algorithm"),
		EncryptedKey: messageString(m, "encrypted_key"),
		IV:           messageString(m, "iv"),
		Text:         messageString(m, textFieldName),
	}
}

// collectSecretMessages 收集消息树中全部目标密文消息实例。
func collectSecretMessages(root protoreflect.Message, target protoreflect.FullName) []protoreflect.Message {
	if !containsSecretTarget(root.Descriptor(), target) {
		return nil
	}
	var out []protoreflect.Message
	appendSecretMessages(root, target, &out)
	return out
}

// appendSecretMessages 递归收集单个消息中的目标密文实例。
func appendSecretMessages(m protoreflect.Message, target protoreflect.FullName, out *[]protoreflect.Message) {
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Kind() != protoreflect.MessageKind || fd.IsMap() {
			continue
		}
		if fd.Message().FullName() == target {
			if fd.IsList() {
				list := m.Get(fd).List()
				for j := 0; j < list.Len(); j++ {
					*out = append(*out, list.Get(j).Message())
				}
				continue
			}
			if m.Has(fd) {
				*out = append(*out, m.Get(fd).Message())
			}
			continue
		}
		if fd.IsList() {
			if !containsSecretTarget(fd.Message(), target) {
				continue
			}
			list := m.Get(fd).List()
			for j := 0; j < list.Len(); j++ {
				appendSecretMessages(list.Get(j).Message(), target, out)
			}
			continue
		}
		if m.Has(fd) && containsSecretTarget(fd.Message(), target) {
			appendSecretMessages(m.Get(fd).Message(), target, out)
		}
	}
}

// containsSecretTarget 判断消息类型是否直接或间接携带目标密文字段，结果按类型缓存。
func containsSecretTarget(md protoreflect.MessageDescriptor, target protoreflect.FullName) bool {
	key := string(target) + "\x00" + string(md.FullName())
	if v, ok := secretTargetMemo.Load(key); ok {
		return v.(bool)
	}
	res := false
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Kind() != protoreflect.MessageKind || fd.IsMap() {
			continue
		}
		fieldMD := fd.Message()
		switch {
		case fieldMD.FullName() == target:
			res = true
		case strings.HasPrefix(string(fieldMD.FullName()), googleWellKnownPrefix):
			// Struct 等已知类型消息不递归扫描。
		default:
			res = containsSecretTarget(fieldMD, target)
		}
		if res {
			break
		}
	}
	secretTargetMemo.Store(key, res)
	return res
}

// messageString 读取消息中指定字符串字段。
func messageString(m protoreflect.Message, name string) string {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		return ""
	}
	return m.Get(fd).String()
}

// setMessageString 写入消息中指定字符串字段。
func setMessageString(m protoreflect.Message, name, value string) {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		return
	}
	m.Set(fd, protoreflect.ValueOfString(value))
}
