# 贡献指南

本文件只描述 `cmd/protoc-gen-go-redact` 子模块的本地开发流程。生成器源码、`redact/v1` Proto 契约和运行时脱敏包位于同一仓库，修改时应同时检查生成链路和对应 Go API。

## 环境

- Go 1.27；
- `protoc`；
- Buf；
- 需要 lint 时安装 `golangci-lint` 和 `staticcheck`。

初始化工具并下载依赖：

```bash
cd cmd/protoc-gen-go-redact
make install-tools
make deps
```

## 日常开发

```bash
make fmt
make test
make vet
make lint
make build
```

`make test` 使用 race detector 并执行全部 Go 测试；`make test-short` 用于快速反馈；`make test-integration` 会先构建插件，再执行集成测试；`make test-coverage` 生成覆盖率 HTML；`make test-bench` 运行基准测试。

构建产物位于 `bin/`，覆盖率产物位于 `coverage/`。清理二进制和覆盖率文件：

```bash
make clean-bin
make clean-coverage
```

## Proto 与生成代码

修改 `redact/v1/redact.proto` 或生成模板后执行：

```bash
make buf-format
make buf-lint
make buf-breaking
make buf-generate
```

`make generate` 是兼容入口，行为与 `make buf-generate` 相同；`make generate-testdata` 会重新生成集成测试数据。提交前检查生成文件是否与 Proto 和模板一致，不要手工编辑生成的 `*.pb.go`。

如需发布 Proto 模块，先执行 `make buf-lint`，再按发布流程执行 `make buf-push` 或 `make buf-push-tag TAG=<tag>`。发布动作需要相应的 Buf 凭据，由维护者在具备凭据的环境执行。

## 测试编写

- 规则解析、字段遍历和模板输出使用单元测试；
- 真实 `protoc` 编译链路放在集成测试；
- 新增字段类型、oneof、optional、map 或递归消息时，应补充成功和错误路径；
- 运行时行为变更要同时检查 `redact` 模块的测试和本命令生成结果。

## 提交前检查

```bash
make fmt
make buf-lint
make test
make vet
```

生成文件有变化时，把 Proto、模板、生成代码和测试一起检查。提交说明应说明行为变化和兼容性影响；不要提交本地 `bin/`、`coverage/` 或临时 Proto 产物。
