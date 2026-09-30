# kratos-kit SecretCrypto

`secretcrypto` 提供敏感字段的临时公钥加密传输能力，避免密码、密钥等明文出现在请求体与访问日志中。

## 工作流程

1. 前端先调用业务接口获取一次性临时 RSA 公钥（`GeneratePublicKey`），私钥记录写入缓存等待消费。
2. 前端生成一次性 AES-256-GCM 对称密钥加密明文，再用 RSA-OAEP 包裹对称密钥，连同 `key_id`、`nonce` 等随请求提交。
3. 服务端中间件在进入业务前统一解密回填明文，业务处理完成后立即清除明文。
4. 私钥记录取出即删，配合较短有效期防止重放；日志中间件只能看到密文。

## 加密格式

- 算法标识：`RSA-OAEP-256+A256GCM`（常量 `Algorithm`）。
- 密文载体 `Cipher` 与 proto 消息 `common.v1.SecretCrypto` 字段一一对应：`key_id`、`nonce`、`algorithm`、`encrypted_key`、`iv`、`text`。
- `text` 字段传输时承载密文，解密后回填明文。
- 同一请求的多个敏感字段应使用同一份公钥加密，服务端按 `key_id` 分组只消费一次私钥记录。

## 服务端接入

```go
import (
    "github.com/liujitcn/kratos-kit/cache"
    "github.com/liujitcn/kratos-kit/secretcrypto"
)

cacheClient, cleanup, err := cache.NewCache(nil)
if err != nil {
    return err
}
defer cleanup()

svc := secretcrypto.NewService(cacheClient)
```

签发一次性临时公钥，返回给前端：

```go
info, err := svc.GeneratePublicKey()
if err != nil {
    return err
}
// info.KeyID / info.PublicKey / info.Algorithm / info.Nonce / info.ExpiresIn
```

把中间件挂在调用链最内侧，保证日志中间件只见密文。请求消息树中全部 `common.v1.SecretCrypto` 类型字段（含嵌套与列表）会在进入业务前解密回填明文：

```go
// kratos HTTP/gRPC server 中间件注册
server := http.NewServer(
    http.Middleware(
        logging.Server(...),   // 日志在外层，只见密文
        recovery.Recovery(),
        svc.Middleware(),      // 最内层
    ),
)
```

不走中间件时也可以直接批量解密：

```go
if err := svc.Decrypt(cipher1, cipher2); err != nil {
    return err
}
// cipher1.Text / cipher2.Text 已回填明文
```

## 可选参数

```go
svc := secretcrypto.NewService(cacheClient,
    secretcrypto.WithTTL(5*time.Minute),                        // 私钥记录有效期，默认 DefaultTTL
    secretcrypto.WithKeyBits(2048),                             // 临时 RSA 密钥位数，默认 DefaultKeyBits
    secretcrypto.WithMessageTypeName("common.v1.SecretCrypto"), // 中间件识别的密文消息全名，默认 DefaultMessageTypeName
    secretcrypto.WithTextFieldName("text"),                     // 承载密文与明文的字段名，默认 DefaultTextFieldName
)
```

## 错误

错误通过 `errors.New(400, reason, message)` 返回，前端按 reason 区分处理：

- `secretcrypto.ReasonInvalid`（`SECRET_CRYPTO_INVALID`）：密文不完整、算法不支持、随机值不符或解密失败。
- `secretcrypto.ReasonKeyExpired`（`SECRET_CRYPTO_KEY_EXPIRED`）：临时私钥不存在或已过期，需重新获取公钥后提交。

## 验证

```bash
cd secretcrypto
go test ./...
go vet ./...
```
