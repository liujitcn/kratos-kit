# Makefile 提供项目初始化、Proto 生成、Wire、检查和启动命令。
.PHONY: help init plugin cli fmt gorm-gen api openapi ts wire gen test run run-only build package-binary docker-build

IMAGE ?= __PROJECT_NAME__
TAG ?= latest
BINARY ?= bin/server
DIST_DIR ?= dist
PACKAGE_NAME ?= backend
SERVER_PACKAGE ?= ./internal/cmd/server
WIRE_DIR ?= internal/cmd/server
GO_BIN_DIR ?= $(shell GOBIN=$$(go env GOBIN); if [ -n "$$GOBIN" ]; then printf '%s' "$$GOBIN"; else printf '%s/bin' "$$(go env GOPATH)"; fi)
GORM_GEN_BIN ?= $(or $(shell command -v gorm-gen 2>/dev/null),$(GO_BIN_DIR)/gorm-gen)
GORM_GEN_CONFIG ?= configs/data.yaml
GORM_TABLE ?=

# 初始化项目生成工具。
init: plugin cli

# 安装 Proto 生成插件。
plugin:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-redact@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-agent-tool@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-mcp-tool@latest

# 安装项目命令行工具。
cli:
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/google/wire/cmd/wire@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/liujitcn/gorm-kit/gen/cmd/gorm-gen@latest

# 格式化 Go 源码。
fmt:
	@PATH="$(GO_BIN_DIR):$$PATH" find . -type f -name '*.go' -exec goimports -w {} +

# 根据数据库表生成 GORM 模型、查询和数据访问代码。
gorm-gen:
	@test -x "$(GORM_GEN_BIN)" || (echo "未找到 gorm-gen，请先执行 make cli" && exit 1)
	@$(GORM_GEN_BIN) -base_path=internal/data/gen -config="$(GORM_GEN_CONFIG)" $(if $(strip $(GORM_TABLE)),-table="$(GORM_TABLE)")

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

# 生成管理端 TypeScript RPC 代码。
ts:
	@cd api && PATH="$(GO_BIN_DIR):$$PATH" buf generate --template buf.admin.business.typescript.gen.yaml

# 生成独立服务入口 Wire 代码。
wire:
	@cd "$(WIRE_DIR)" && PATH="$(GO_BIN_DIR):$$PATH" wire .

# 生成当前项目全部协议、OpenAPI 和 Wire 产物。
gen: gorm-gen api openapi ts wire fmt

# 验证项目编译和依赖完整性。
test:
	@go test ./...

# 使用已有生成产物启动后端服务。
run-only:
	@go run "$(SERVER_PACKAGE)" --conf ./configs

# 刷新 Wire 后启动后端服务。
run: wire run-only

# 构建 Linux amd64 可执行文件。
build: wire
	@mkdir -p $(dir $(BINARY))
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY) $(SERVER_PACKAGE)

# 打包后端二进制、配置和 Docker 启动脚本。
package-binary: build
	@mkdir -p $(DIST_DIR)
	@tar -czf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz $(BINARY) configs scripts

# 构建后端 Docker 镜像。
docker-build:
	@docker build -t $(IMAGE):$(TAG) .

# 显示帮助。
help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets: init fmt gorm-gen api openapi ts wire gen test run run-only build package-binary docker-build"

.DEFAULT_GOAL := help
