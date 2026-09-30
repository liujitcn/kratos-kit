package secretcrypto

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/liujitcn/go-utils/crypto"
	"github.com/liujitcn/kratos-kit/cache"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const testPlaintext = "机密数据sk-test-123"

// newTestFile 构建测试用的动态 proto 文件描述符。
func newTestFile(t *testing.T) protoreflect.FileDescriptor {
	t.Helper()
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test_secret.proto"),
		Package: proto.String("common.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("SecretCrypto"), Field: secretCryptoFields()},
			{Name: proto.String("Holder"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("secret"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".common.v1.SecretCrypto")},
				{Name: proto.String("items"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".common.v1.SecretCrypto")},
				{Name: proto.String("nested"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".common.v1.Nested")},
			}},
			{Name: proto.String("Nested"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("secret"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".common.v1.SecretCrypto")},
			}},
		},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("构建测试描述符: %v", err)
	}
	return fd
}

// secretCryptoFields 构建与 common.v1.SecretCrypto 同构的字段列表。
func secretCryptoFields() []*descriptorpb.FieldDescriptorProto {
	names := []struct {
		name   string
		number int32
	}{
		{"key_id", 1}, {"nonce", 2}, {"algorithm", 3},
		{"encrypted_key", 4}, {"iv", 5}, {"text", 6},
	}
	fields := make([]*descriptorpb.FieldDescriptorProto, 0, len(names))
	for _, item := range names {
		fields = append(fields, &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(item.name),
			Number: proto.Int32(item.number),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		})
	}
	return fields
}

// newTestService 创建使用内存缓存的服务。
func newTestService(t *testing.T) *Service {
	t.Helper()
	cacheClient, cleanup, err := cache.NewCache(nil)
	if err != nil {
		t.Fatalf("创建测试缓存: %v", err)
	}
	t.Cleanup(cleanup)
	return NewService(cacheClient)
}

// encryptToCipher 使用公钥信息构建单个密文字段。
func encryptToCipher(t *testing.T, info *PublicKeyInfo, plaintext string) *Cipher {
	t.Helper()
	publicCrypto, err := crypto.NewRSACryptoFromPublicKeyPEM(info.PublicKey)
	if err != nil {
		t.Fatalf("解析临时公钥: %v", err)
	}
	aesKey, err := crypto.GenerateAESKey(32)
	if err != nil {
		t.Fatalf("生成对称密钥: %v", err)
	}
	iv := make([]byte, 12)
	if _, err = rand.Read(iv); err != nil {
		t.Fatalf("生成初始向量: %v", err)
	}
	ciphertext, err := crypto.AesGCMEncrypt([]byte(plaintext), aesKey, iv)
	if err != nil {
		t.Fatalf("加密明文: %v", err)
	}
	encryptedKey, err := publicCrypto.EncryptBytes(aesKey)
	if err != nil {
		t.Fatalf("包裹对称密钥: %v", err)
	}
	return &Cipher{
		KeyID:        info.KeyID,
		Nonce:        info.Nonce,
		Algorithm:    info.Algorithm,
		EncryptedKey: encryptedKey,
		IV:           base64.StdEncoding.EncodeToString(iv),
		Text:         base64.StdEncoding.EncodeToString(ciphertext),
	}
}

// fillSecretMessage 将密文写入动态消息字段。
func fillSecretMessage(t *testing.T, m protoreflect.Message, item *Cipher) {
	t.Helper()
	fields := m.Descriptor().Fields()
	values := map[string]string{
		"key_id": item.KeyID, "nonce": item.Nonce, "algorithm": item.Algorithm,
		"encrypted_key": item.EncryptedKey, "iv": item.IV, "text": item.Text,
	}
	for name, value := range values {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			t.Fatalf("缺少字段 %s", name)
		}
		m.Set(fd, protoreflect.ValueOfString(value))
	}
}

func TestDecryptBatchSharesKeyRecord(t *testing.T) {
	svc := newTestService(t)
	info, err := svc.GeneratePublicKey()
	if err != nil {
		t.Fatalf("签发公钥: %v", err)
	}
	first := encryptToCipher(t, info, testPlaintext)
	second := encryptToCipher(t, info, "第二个机密")
	if err = svc.Decrypt(first, second); err != nil {
		t.Fatalf("批量解密: %v", err)
	}
	if first.Text != testPlaintext || second.Text != "第二个机密" {
		t.Fatalf("解密结果不符: %q %q", first.Text, second.Text)
	}
	third := encryptToCipher(t, info, "重放")
	if err = svc.Decrypt(third); err == nil {
		t.Fatal("私钥记录取出即删，重放应当失败")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	svc := newTestService(t)
	info, err := svc.GeneratePublicKey()
	if err != nil {
		t.Fatalf("签发公钥: %v", err)
	}
	item := encryptToCipher(t, info, testPlaintext)
	item.Text = "tampered"
	if err = svc.Decrypt(item); err == nil {
		t.Fatal("篡改密文应当失败")
	}
}

func TestMiddlewareDecryptsAndClears(t *testing.T) {
	fd := newTestFile(t)
	holderDesc := fd.Messages().ByName("Holder")
	secretDesc := fd.Messages().ByName("SecretCrypto")

	svc := newTestService(t)
	info, err := svc.GeneratePublicKey()
	if err != nil {
		t.Fatalf("签发公钥: %v", err)
	}

	holder := dynamicpb.NewMessage(holderDesc)
	secret := dynamicpb.NewMessage(secretDesc)
	fillSecretMessage(t, secret, encryptToCipher(t, info, testPlaintext))
	holder.Set(holder.Descriptor().Fields().ByName("secret"), protoreflect.ValueOfMessage(secret))

	var seen string
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		seen = req.(proto.Message).ProtoReflect().
			Get(req.(proto.Message).ProtoReflect().Descriptor().Fields().ByName("secret")).
			Message().Get(secretDesc.Fields().ByName("text")).String()
		return "ok", nil
	}

	resp, err := svc.Middleware()(handler)(context.Background(), holder)
	if err != nil {
		t.Fatalf("中间件执行: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("响应不符: %v", resp)
	}
	if seen != testPlaintext {
		t.Fatalf("业务内明文不符: %q", seen)
	}
	cleared := holder.Get(holder.Descriptor().Fields().ByName("secret")).Message().
		Get(secretDesc.Fields().ByName("text")).String()
	if cleared != "" {
		t.Fatalf("处理完成后明文应被清除: %q", cleared)
	}
}

func TestMiddlewareRejectsExpiredKey(t *testing.T) {
	fd := newTestFile(t)
	holderDesc := fd.Messages().ByName("Holder")
	secretDesc := fd.Messages().ByName("SecretCrypto")

	svc := newTestService(t)
	holder := dynamicpb.NewMessage(holderDesc)
	secret := dynamicpb.NewMessage(secretDesc)
	item := &Cipher{
		KeyID: "missing", Nonce: "x", Algorithm: Algorithm,
		EncryptedKey: "a", IV: base64.StdEncoding.EncodeToString([]byte("iv")), Text: base64.StdEncoding.EncodeToString([]byte("ct")),
	}
	fillSecretMessage(t, secret, item)
	holder.Set(holder.Descriptor().Fields().ByName("secret"), protoreflect.ValueOfMessage(secret))

	stub := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }
	if _, err := svc.Middleware()(stub)(context.Background(), holder); err == nil {
		t.Fatal("私钥记录不存在时应当失败")
	}
}
