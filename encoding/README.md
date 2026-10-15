# encoding

`encoding` 提供可直接注册到 Kratos codec 工厂的额外编解码模块。每个编码格式是独立 Go module，按需引入即可。

## 支持格式

| 模块 | 格式 |
| --- | --- |
| `encoding/avro` | Apache Avro，需要传入 schema。 |
| `encoding/bson` | BSON。 |
| `encoding/cbor` | CBOR。 |
| `encoding/flatbuffers` | FlatBuffers。 |
| `encoding/gob` | Go Gob。 |
| `encoding/thrift` | Thrift。 |
| `encoding/toml` | TOML。 |

各模块实现 Kratos `encoding.Codec`，并提供 `Name`、`Marshal`、`Unmarshal` 等标准方法；Avro 额外提供 `NewCodec(schema)` 创建带 schema 的 codec。导入对应模块后，将 codec 注册到服务端或客户端的 `encoding` 配置即可。

```bash
go get github.com/liujitcn/kratos-kit/encoding/cbor@latest
```

`msgpack`、`xml`、`yaml` 等格式使用 Kratos v3 自带实现，不在本目录重复封装。
