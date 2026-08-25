# 运行要求：Linux/macOS，或 Windows 下使用 WSL/Git Bash（需具备 make、python3、go）

.PHONY: help init plugin cli fmt api gen tag

# 初始化开发环境
init: plugin cli

# 安装 protoc 插件
plugin:
	# Go
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest
	@go install github.com/menta2k/protoc-gen-redact/v3@latest
	@go install github.com/go-kratos/protoc-gen-typescript-http@latest
	@cd cmd/protoc-gen-go-agent-tool && go install .
	@cd cmd/protoc-gen-go-mcp-tool && go install .
	@cd cmd/protoc-gen-go-service && go install .

# 安装命令行工具
cli:
	@go install github.com/go-kratos/kratos/cmd/kratos/v3@latest
	@go install github.com/google/gnostic@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install entgo.io/ent/cmd/ent@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install ./cmd/normalize-go-imports
	@cd cmd/project-docs && go install .
	@cd cmd/kratos-admin-backend && go install .

# 使用 goimports 统一整理 Go 代码的 import 与格式
fmt:
	@normalize_bin="$$(go env GOBIN)"; \
	if [ -z "$$normalize_bin" ]; then normalize_bin="$$(go env GOPATH)/bin"; fi; \
	"$$normalize_bin/normalize-go-imports" -root . -write
	@goimports -w $$(rg --files -g '*.go')

# 生成 protobuf API Go 代码
api:
	@cd api && \
	buf generate

# 一键生成全部接口产物（Go 代码）
gen: api fmt

# 统一打 tag：默认递归扫描；MODULE 指定起始目录，EXACT=1 时仅处理该 module（不提交代码）
tag:
	@python3 scripts/tag_release.py $(if $(MODULE),--path $(MODULE),) $(if $(EXACT),--exact,)

# 显示帮助
help:
	@echo ""
	@echo "Usage:"
	@echo " make [target]"
	@echo ""
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
