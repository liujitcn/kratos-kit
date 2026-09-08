# Backend Makefile
#
# 常用流程：
#   首次准备：make init
#   日常启动：make run
#   仅启动服务：make run-only
#   后端生成：make gen
#   提交前检查：make check
#   Linux 二进制：make build

.PHONY: help init plugin cli fmt lint test check \
	gorm-gen api openapi public-wire wire gen \
	run run-only build package-binary

# ===== 运行与构建参数 =====

# 独立后端始终使用自己的 go.mod，避免继承调用方的 Go workspace。
export GOWORK := off

# 服务入口、配置目录、运行环境和附加运行参数。
SERVER_PACKAGE ?= ./internal/cmd/server
CONF ?= ./configs
APP_ENV ?= dev
RUN_ARGS ?=

# 二进制目标平台与输出路径，默认构建 Linux amd64 静态程序。
CGO_ENABLED ?= 0
GOOS ?= linux
GOARCH ?= amd64
BUILD_FLAGS ?=
BINARY ?= bin/server

# 压缩包输出参数，仓库根目录的 make package 会调用此目标。
DIST_DIR ?= dist
PACKAGE_NAME ?= __PROJECT_NAME__-backend-$(GOOS)-$(GOARCH)
ARCHIVE ?= $(DIST_DIR)/$(PACKAGE_NAME).tar.gz

# ===== 工具路径参数 =====

GO_BIN_DIR ?= $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
PUBLIC_WIRE_DIR ?= internal/module
WIRE_DIR ?= internal/cmd/server

# ===== GORM 生成参数 =====

GORM_GEN_BIN ?= $(or \
	$(shell command -v gorm-gen 2>/dev/null), \
	$(GO_BIN_DIR)/gorm-gen \
)
GORM_GEN_CONFIG ?= configs/data.yaml
GORM_GEN_DATABASE ?=
# 仅生成本仓库使用的表；新增业务表时同步补充此列表。
GORM_TABLE ?=
PROTO_FILES := $(shell find api/proto -type f -name '*.proto' 2>/dev/null)

# ===== 环境初始化 =====

# 按顺序安装 protoc 插件和开发命令行工具。
init:
	@$(MAKE) plugin
	@$(MAKE) cli

# 安装 protoc 及项目自定义生成插件。
plugin:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest
	@go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-redact@latest
	@if [ -d ../../kratos-kit/cmd/protoc-gen-go-agent-tool ]; then \
		cd ../../kratos-kit/cmd/protoc-gen-go-agent-tool && go install .; \
	else \
		go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-agent-tool@latest; \
	fi
	@if [ -d ../../kratos-kit/cmd/protoc-gen-go-mcp-tool ]; then \
		cd ../../kratos-kit/cmd/protoc-gen-go-mcp-tool && go install .; \
	else \
		go install github.com/liujitcn/kratos-kit/cmd/protoc-gen-go-mcp-tool@latest; \
	fi

# 安装日常开发、检查和代码生成命令行工具。
cli:
	@go install github.com/go-kratos/kratos/cmd/kratos/v3@latest
	@go install github.com/google/gnostic@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/google/wire/cmd/wire@v0.7.0
	@go install entgo.io/ent/cmd/ent@latest
	@go install golang.org/x/tools/gopls@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@GOBIN="$(GO_BIN_DIR)" go install github.com/liujitcn/kratos-kit/cmd/normalize-go-imports@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@if [ -d ../../gorm-kit/gen/cmd/gorm-gen ]; then \
		cd ../../gorm-kit/gen && go install ./cmd/gorm-gen; \
	else \
		go install github.com/liujitcn/gorm-kit/gen/cmd/gorm-gen@latest; \
	fi

# ===== 格式化与检查 =====

# 使用 goimports 和共享工具统一整理 Go 代码。
fmt:
	@echo "==> 格式化 Go 代码"
	@PATH="$(GO_BIN_DIR):$$PATH" find . -type f -name '*.go' -exec goimports -w {} +
	@"$(GO_BIN_DIR)/normalize-go-imports" -root . -write
	@echo "==> Go 代码格式化完成"

# 运行 golangci-lint 静态检查（含禁止手写 SQL 检查）。
lint:
	@echo "==> 运行 golangci-lint 静态检查"
	@PATH="$(GO_BIN_DIR):$$PATH" golangci-lint run ./...
	@echo "==> golangci-lint 静态检查完成"

# 运行 Backend 全量 Go 测试。
test:
	@echo "==> 运行 Go 测试"
	@go test ./...
	@echo "==> Go 测试完成"

# 按顺序执行静态检查和 Go 测试。
check:
	@$(MAKE) lint
	@$(MAKE) test

# ===== 代码生成 =====

# 根据数据库表生成 GORM 模型、查询和数据访问代码，生成产物不保留数据库默认值。
gorm-gen:
ifneq ($(strip $(GORM_TABLE)),)
	@echo "==> 生成 GORM 代码"
	@test -x "$(GORM_GEN_BIN)" || (echo "未找到 gorm-gen，请先执行 make cli" && exit 1)
	@$(GORM_GEN_BIN) -base_path=internal/data/gen -config='$(GORM_GEN_CONFIG)' $(if $(strip $(GORM_GEN_DATABASE)),-database='$(GORM_GEN_DATABASE)') $(if $(strip $(GORM_TABLE)),-table='$(GORM_TABLE)')
	@PATH="$(GO_BIN_DIR):$$PATH" find internal/data/gen -type f -name '*.go' -exec goimports -w {} +
	@find internal/data/gen -type f -name '*.go' -exec perl -pi -e 's/\x54\x72\x61\x6e\x73\x6c\x61\x74\x69\x6f\x6eTargetType/I18nTargetType/g' {} +
	@find internal/data/gen -type f -name '*.go' -exec perl -pi -e 's/;default:[^;"`]+//gi' {} +
	@echo "==> GORM 代码生成完成"
else
	@echo "未配置业务表，跳过 GORM 生成"
endif

# 根据 Proto 契约生成 Backend protobuf Go 代码。
api:
ifneq ($(strip $(PROTO_FILES)),)
	@echo "==> 生成 protobuf Go 代码"
	@cd api && \
		PATH="$(GO_BIN_DIR):$$PATH" buf generate --template buf.gen.yaml
	@PATH="$(GO_BIN_DIR):$$PATH" find api/gen/go -type f -name '*.go' -exec goimports -w {} +
	@find api/gen/go -type f -name '*.go' -exec perl -pi -e 's/BaseI18N/BaseI18n/g; s/baseI18N/baseI18n/g; s/I18NTargetType/I18nTargetType/g; s/I18Ns/I18ns/g; s/\bI18N\b/I18n/g' {} +
	@echo "==> protobuf Go 代码生成完成"
else
	@echo "未发现业务 Proto，跳过 Go 协议生成"
endif

# 根据 Proto 契约生成 OpenAPI v3 源文档；多语言文档由仓库根目录生成。
openapi:
ifneq ($(strip $(PROTO_FILES)),)
	@echo "==> 生成 OpenAPI v3 文档"
	@cd api && \
		PATH="$(GO_BIN_DIR):$$PATH" buf generate --template buf.openapi.gen.yaml
	@find internal/openapi/assets -type f -name '*.yaml' -exec perl -pi -e 's/BaseI18N/BaseI18n/g; s/baseI18N/baseI18n/g' {} +
	@echo "==> OpenAPI v3 源文档生成完成"
else
	@echo "未发现业务 Proto，保留空 OpenAPI 资源"
endif

# 生成 Backend 公共入口使用的内部 Wire 依赖注入代码。
public-wire:
	@echo "==> 生成 Backend 公共入口使用的内部 Wire 依赖注入代码"
	@cd "$(PUBLIC_WIRE_DIR)" && PATH="$(GO_BIN_DIR):$$PATH" wire . && \
		PATH="$(GO_BIN_DIR):$$PATH" goimports -w wire.go wire_gen.go && \
		perl -0pi -e 's/\n\t"github.com\/liujitcn\/kratos-kit\/api\/gen\/go\/config\/v1"\n/\n/' wire_gen.go && \
		echo "==> Backend 公共入口使用的内部 Wire 依赖注入代码生成完成"

# 生成独立服务入口 Wire 依赖注入代码。
wire:
	@if [ ! -f "$(WIRE_DIR)/wire.go" ]; then \
		echo "==> 未找到 $(WIRE_DIR)/wire.go，跳过 Wire 依赖注入代码生成"; \
	else \
		echo "==> 生成独立服务入口 Wire 依赖注入代码"; \
		cd "$(WIRE_DIR)" && PATH="$(GO_BIN_DIR):$$PATH" wire . && \
		PATH="$(GO_BIN_DIR):$$PATH" goimports -w wire.go wire_gen.go && \
		echo "==> 独立服务入口 Wire 依赖注入代码生成完成"; \
	fi

# 按 GORM、后端接口、Wire、格式化的顺序生成全部后端产物。
gen:
	@$(MAKE) gorm-gen
	@$(MAKE) api
	@$(MAKE) openapi
	@$(MAKE) public-wire
	@$(MAKE) wire
	@$(MAKE) fmt
	@echo "==> 全部后端代码产物生成完成"

# ===== 运行与构建 =====

# 直接启动服务，不执行代码生成。
run-only:
	@go run "$(SERVER_PACKAGE)" -c "$(CONF)" -e "$(APP_ENV)" $(RUN_ARGS)

# 刷新接口、OpenAPI 和 Wire 后运行服务。
run:
	@$(MAKE) api
	@$(MAKE) openapi
	@$(MAKE) wire
	@$(MAKE) run-only

# 刷新 Wire 后构建目标平台可执行文件。
build:
	@$(MAKE) wire
	@mkdir -p "$(dir $(BINARY))"
	@CGO_ENABLED="$(CGO_ENABLED)" GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build $(BUILD_FLAGS) -o "$(BINARY)" "$(SERVER_PACKAGE)"
	@echo "==> 可执行文件已生成: $(BINARY) ($(GOOS)/$(GOARCH))"

# 构建并打包后端二进制与运行配置，供仓库根目录的 package 调用。
package-binary:
	@$(MAKE) build
	@mkdir -p "$(dir $(ARCHIVE))"
	@tar -czf "$(ARCHIVE)" "$(BINARY)" configs
	@echo "==> 后端发布压缩包已生成: $(ARCHIVE)"

# ===== 帮助 =====

# 查看 Backend 目标及可覆盖参数。
help:
	@echo ""
	@echo "常用流程:"
	@echo "  make init                         首次安装生成与检查工具"
	@echo "  make run                          刷新接口、OpenAPI 和 Wire 后启动服务"
	@echo "  make run-only                     已有生成产物时直接启动服务"
	@echo "  make gen                          生成后端代码产物"
	@echo "  make check                        提交前执行 lint 和测试"
	@echo "  make build                        构建 Linux amd64 二进制"
	@echo ""
	@echo "可用目标:"
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m  %-20s\033[0m %s\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "常用参数（命令行使用 参数=值 覆盖）:"
	@printf "  %-24s %s\n" "CONF" "运行配置目录，当前: $(CONF)"
	@printf "  %-24s %s\n" "APP_ENV" "运行环境，当前: $(APP_ENV)"
	@printf "  %-24s %s\n" "GOOS / GOARCH" "目标平台，当前: $(GOOS) / $(GOARCH)"
	@printf "  %-24s %s\n" "BINARY" "二进制输出，当前: $(BINARY)"
	@printf "  %-24s %s\n" "ARCHIVE" "后端压缩包输出，当前: $(ARCHIVE)"
	@printf "  %-24s %s\n" "PUBLIC_WIRE_DIR" "公共入口内部 Wire 目录，当前: $(PUBLIC_WIRE_DIR)"
	@printf "  %-24s %s\n" "WIRE_DIR" "独立入口 Wire 目录，当前: $(WIRE_DIR)"
	@printf "  %-24s %s\n" "GORM_GEN_CONFIG" "GORM 数据源配置，当前: $(GORM_GEN_CONFIG)"
	@printf "  %-24s %s\n" "GORM_GEN_DATABASE" "可选数据库名，默认使用配置文件"
	@printf "  %-24s %s\n" "GORM_TABLE" "可选逗号分隔表清单"
	@echo ""
	@echo "示例:"
	@echo "  make run-only CONF=./configs APP_ENV=dev"
	@echo "  make run-only RUN_ARGS='--help'"
	@echo "  make build GOOS=darwin GOARCH=arm64 BINARY=bin/server-darwin-arm64"
	@echo "  make package-binary GOOS=linux GOARCH=arm64"

.DEFAULT_GOAL := help
