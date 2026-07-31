# Makefile 提供项目初始化、生成、检查和启动命令。
.PHONY: help init plugin cli fmt api openapi project-docs wire gen test run build docker-build

IMAGE ?= __PROJECT_NAME__
TAG ?= latest
BINARY ?= bin/server
PROJECT_KEY ?= __PROJECT_NAME__
PROJECT_NAME ?= __PROJECT_NAME__
PROJECT_DOCS_BIN ?= project-docs
PROJECT_DOC_ARGS ?= --source '$(PROJECT_KEY):$(PROJECT_NAME)=.'
GO_BIN_DIR ?= $(shell GOBIN=$$(go env GOBIN); if [ -n "$$GOBIN" ]; then printf '%s' "$$GOBIN"; else printf '%s/bin' "$$(go env GOPATH)"; fi)

# 初始化项目生成工具。
init: plugin cli

# 安装 Proto 生成插件。
plugin:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-agent-tool@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-mcp-tool@latest

# 安装项目命令行工具。
cli:
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/google/wire/cmd/wire@latest
	@go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
	@go install golang.org/x/tools/cmd/goimports@latest

# 格式化 Go 源码。
fmt:
	@PATH="$(GO_BIN_DIR):$$PATH" find . -type f -name '*.go' -exec goimports -w {} +

# 生成 Proto Go 代码。
api:
	@if [ -z "$$(find api/proto -type f -name '*.proto' -print -quit)" ]; then \
		echo "未发现 Proto 文件，跳过 API 生成"; \
	else \
		cd api && PATH="$(GO_BIN_DIR):$$PATH" buf generate --template buf.gen.yaml; \
	fi

# 生成 OpenAPI 文档。
openapi:
	@if [ -z "$$(find api/proto -type f -name '*.proto' -print -quit)" ]; then \
		echo "未发现 Proto 文件，跳过 OpenAPI 生成"; \
	else \
		cd api && PATH="$(GO_BIN_DIR):$$PATH" buf generate --template buf.openapi.gen.yaml; \
	fi

# 收集 README 和 docs 项目文档。
project-docs:
	@command -v "$(PROJECT_DOCS_BIN)" >/dev/null 2>&1 || (echo "project-docs not found. Run make cli first." && exit 1)
	@"$(PROJECT_DOCS_BIN)" $(PROJECT_DOC_ARGS)

# 生成 Wire 依赖注入代码。
wire:
	@PATH="$(GO_BIN_DIR):$$PATH" wire .

# 生成当前项目全部构建产物。
gen: project-docs api openapi wire fmt

# 验证项目编译和依赖完整性。
test:
	@go test ./...

# 启动后端服务。
run: project-docs
	@go run ./internal/cmd/server --conf ./configs

# 构建 Linux 可执行文件。
build: project-docs
	@mkdir -p $(dir $(BINARY))
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY) ./internal/cmd/server

# 构建后端 Docker 镜像。
docker-build:
	@docker build -t $(IMAGE):$(TAG) .

# 显示帮助。
help:
	@echo ""
	@echo "Usage:"
	@echo " make [target]"
	@echo ""
	@echo "Targets: init fmt api openapi project-docs wire gen test run build docker-build"

.DEFAULT_GOAL := help
