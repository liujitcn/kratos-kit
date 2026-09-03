# Frontend Makefile
#
# 常用流程：
#   安装全部依赖：make init
#   启动全部前端：make run
#   检查全部前端：make check
#   生成全部前端 RPC：make ts
#   构建全部宿主：make build
#   生成全部 npm 包：make package

.PHONY: help init reinstall pnpm-admin pnpm-uni-app pnpm-taro-app \
	ts types-admin ts-admin ts-uni-app ts-taro-app \
	run run-all run-h5 run-mp-weixin run-admin run-uni-app run-uni-app-h5 run-uni-app-mp-weixin run-taro-app run-taro-app-h5 run-taro-app-mp-weixin \
	check check-admin check-uni-app check-taro-app \
	build build-all build-h5 build-mp-weixin build-admin build-uni-app build-uni-app-h5 build-uni-app-mp-weixin build-taro-app build-taro-app-h5 build-taro-app-mp-weixin build-package \
	package package-admin package-uni-app package-taro-app \
	publish publish-only publish-admin publish-uni-app publish-taro-app

# ===== 执行参数 =====

PNPM ?= pnpm
CI ?= 1
GO_BIN_DIR ?= $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
API_DIR ?= ../backend/api

TS_PROTO_BIN ?= $(or \
	$(shell command -v protoc-gen-ts_proto 2>/dev/null), \
	$(wildcard $(CURDIR)/admin/node_modules/.bin/protoc-gen-ts_proto), \
	$(wildcard $(CURDIR)/uni-app/node_modules/.bin/protoc-gen-ts_proto), \
	$(wildcard $(CURDIR)/taro-app/node_modules/.bin/protoc-gen-ts_proto), \
	$(lastword $(wildcard $(HOME)/.nvm/versions/node/*/bin/protoc-gen-ts_proto)) \
)
TS_PROTO_BIN_DIR := $(dir $(TS_PROTO_BIN))

# ===== npm 发布参数 =====

NPM_REGISTRY ?= https://registry.npmjs.org/
NPM_ACCESS ?= public
NPM_TAG ?= latest
NPM_PACK_DEST ?= dist/npm
NPM_SKIP_GIT_CHECKS ?= true

NPM_PUBLISH_ENV := NPM_REGISTRY="$(NPM_REGISTRY)" NPM_ACCESS="$(NPM_ACCESS)" NPM_TAG="$(NPM_TAG)" NPM_SKIP_GIT_CHECKS="$(NPM_SKIP_GIT_CHECKS)"

# ===== 依赖安装 =====

# 按管理后台、uni-app、Taro 的顺序安装全部前端依赖
init:
	@$(MAKE) pnpm-admin
	@$(MAKE) pnpm-uni-app
	@$(MAKE) pnpm-taro-app

# 删除全部前端 node_modules 并重新安装依赖
reinstall:
	@bash scripts/reinstall_dependencies.sh

# 安装管理后台依赖
pnpm-admin:
	@cd admin && CI="$(CI)" $(PNPM) install

# 安装 uni-app 应用端依赖
pnpm-uni-app:
	@cd uni-app && CI="$(CI)" $(PNPM) install

# 安装 Taro React 应用端依赖
pnpm-taro-app:
	@cd taro-app && CI="$(CI)" $(PNPM) install

# ===== TypeScript RPC 生成 =====

# 生成管理端、uni-app 和 Taro 的全部 TypeScript RPC 代码。
ts: ts-admin ts-uni-app ts-taro-app
	@echo "==> 全部 TypeScript RPC 代码生成完成"

# 生成管理端 TypeScript RPC 代码。
ts-admin: types-admin
	@echo "==> 生成管理端 TypeScript RPC 代码"
	@test -n "$(TS_PROTO_BIN)" || (echo "未找到 protoc-gen-ts_proto，请先执行 make init，或全局安装 ts-proto" && exit 1)
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.admin.typescript.gen.yaml --exclude-path proto/system/app
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.admin.core.typescript.gen.yaml --exclude-path proto/system/app
	@echo "==> 管理端 TypeScript RPC 代码生成完成"

# 生成管理端自动导入声明。
types-admin:
	@echo "==> 生成管理端自动导入声明"
	@if [ -f admin/scripts/generate-auto-import-types.mjs ]; then \
		cd admin && CI="$(CI)" $(PNPM) run generate:auto-import-types; \
	else \
		echo "未发现自动导入声明脚本，跳过自动导入声明生成"; \
	fi
	@echo "==> 管理端自动导入声明生成完成"

# 生成 uni-app 端 TypeScript RPC 代码。
ts-uni-app:
	@echo "==> 生成 uni-app TypeScript RPC 代码"
	@test -n "$(TS_PROTO_BIN)" || (echo "未找到 protoc-gen-ts_proto，请先执行 make init，或全局安装 ts-proto" && exit 1)
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.uni-app.typescript.gen.yaml --exclude-path proto/system/admin
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.uni-app.core.typescript.gen.yaml --exclude-path proto/system/admin
	@echo "==> uni-app TypeScript RPC 代码生成完成"

# 生成 Taro React 端 TypeScript RPC 代码。
ts-taro-app:
	@echo "==> 生成 Taro React TypeScript RPC 代码"
	@test -n "$(TS_PROTO_BIN)" || (echo "未找到 protoc-gen-ts_proto，请先执行 make init，或全局安装 ts-proto" && exit 1)
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.taro-app.typescript.gen.yaml --exclude-path proto/system/admin
	@cd "$(API_DIR)" && \
		PATH="$(TS_PROTO_BIN_DIR):$(GO_BIN_DIR):$$PATH" buf generate --template buf.taro-app.core.typescript.gen.yaml --exclude-path proto/system/admin
	@echo "==> Taro React TypeScript RPC 代码生成完成"

# ===== 本地运行 =====

# 并行启动管理后台、uni-app 和 Taro 的 H5/微信小程序开发环境
run:
	@$(MAKE) run-all

# 并行启动全部前端开发环境；按 Ctrl-C 会同时停止所有子进程
run-all:
	@set -e; \
	pids=""; \
	cleanup() { trap - INT TERM EXIT; test -z "$$pids" || kill $$pids 2>/dev/null || true; test -z "$$pids" || wait $$pids 2>/dev/null || true; }; \
	trap cleanup INT TERM EXIT; \
	$(MAKE) run-admin & pids="$$pids $$!"; \
	$(MAKE) run-uni-app-h5 & pids="$$pids $$!"; \
	$(MAKE) run-uni-app-mp-weixin & pids="$$pids $$!"; \
	$(MAKE) run-taro-app-h5 & pids="$$pids $$!"; \
	$(MAKE) run-taro-app-mp-weixin & pids="$$pids $$!"; \
	wait

# 并行启动全部 H5 开发环境
run-h5:
	@$(MAKE) --jobs=3 run-admin run-uni-app-h5 run-taro-app-h5

# 并行启动全部微信小程序开发环境
run-mp-weixin:
	@$(MAKE) --jobs=2 run-uni-app-mp-weixin run-taro-app-mp-weixin

# 启动管理后台开发服务
run-admin:
	@cd admin && CI="$(CI)" $(PNPM) run dev

# 启动 uni-app 微信小程序开发服务
run-uni-app:
	@$(MAKE) run-uni-app-mp-weixin

# 启动 uni-app H5 开发服务
run-uni-app-h5:
	@cd uni-app && CI="$(CI)" $(PNPM) run dev:h5

# 启动 uni-app 微信小程序开发服务
run-uni-app-mp-weixin:
	@cd uni-app && CI="$(CI)" $(PNPM) run dev:mp-weixin

# 启动 Taro 微信小程序开发服务
run-taro-app:
	@$(MAKE) run-taro-app-mp-weixin

# 启动 Taro H5 开发服务
run-taro-app-h5:
	@cd taro-app && CI="$(CI)" $(PNPM) run dev:h5

# 启动 Taro 微信小程序开发服务
run-taro-app-mp-weixin:
	@cd taro-app && CI="$(CI)" $(PNPM) run dev:mp-weixin

# ===== 代码检查 =====

# 检查管理后台的静态规则、类型和测试
check-admin:
	@cd admin && \
		CI="$(CI)" $(PNPM) exec oxlint --vue-plugin --config ./internal/lint-config/oxlint.json --ignore-path .oxlintignore ./apps ./packages ./internal ./scripts && \
		CI="$(CI)" $(PNPM) run type:check && \
		CI="$(CI)" $(PNPM) run test

# 检查 uni-app 的静态规则、类型、测试和包边界
check-uni-app:
	@cd uni-app && \
		CI="$(CI)" $(PNPM) exec eslint . && \
		CI="$(CI)" $(PNPM) run tsc && \
		CI="$(CI)" $(PNPM) run test && \
		CI="$(CI)" $(PNPM) run check:exports

# 检查 Taro 的静态规则、类型、测试和包边界
check-taro-app:
	@cd taro-app && \
		CI="$(CI)" $(PNPM) exec eslint . --ext .js,.mjs,.ts,.tsx && \
		CI="$(CI)" $(PNPM) run tsc && \
		CI="$(CI)" $(PNPM) run test && \
		CI="$(CI)" $(PNPM) run check:exports

# 按管理后台、uni-app、Taro 的顺序检查全部前端
check:
	@$(MAKE) check-admin
	@$(MAKE) check-uni-app
	@$(MAKE) check-taro-app
	@echo "==> 全部前端检查完成"

# ===== 应用与模块构建 =====

# 构建全部前端 H5 和微信小程序产物
build:
	@$(MAKE) build-all

# 构建管理后台静态资源
build-admin:
	@cd admin && CI="$(CI)" $(PNPM) run build

# 构建 uni-app H5 静态资源
build-uni-app:
	@$(MAKE) build-uni-app-h5

# 构建 uni-app H5 静态资源
build-uni-app-h5:
	@cd uni-app && CI="$(CI)" $(PNPM) run build:h5

# 构建 uni-app 微信小程序产物
build-uni-app-mp-weixin:
	@cd uni-app && CI="$(CI)" $(PNPM) run build:mp-weixin

# 构建 Taro H5 静态资源
build-taro-app:
	@$(MAKE) build-taro-app-h5

# 构建 Taro H5 静态资源
build-taro-app-h5:
	@cd taro-app && CI="$(CI)" $(PNPM) run build:h5

# 构建 Taro 微信小程序产物
build-taro-app-mp-weixin:
	@cd taro-app && CI="$(CI)" $(PNPM) run build:mp-weixin

# 构建管理后台、uni-app 和 Taro 的全部 H5 产物
build-h5:
	@$(MAKE) build-admin
	@$(MAKE) build-uni-app-h5
	@$(MAKE) build-taro-app-h5

# 构建 uni-app 和 Taro 的全部微信小程序产物
build-mp-weixin:
	@$(MAKE) build-uni-app-mp-weixin
	@$(MAKE) build-taro-app-mp-weixin

# 构建管理后台、uni-app 和 Taro 的 H5/微信小程序全部宿主
build-all:
	@$(MAKE) build-h5
	@$(MAKE) build-mp-weixin
	@echo "==> 全部前端宿主构建完成"

# 构建可供业务后台组合使用的管理端模块包
build-package:
	@cd admin && CI="$(CI)" $(PNPM) run build:package

# ===== npm 打包 =====

# 检查并生成管理后台 npm 包
package-admin:
	@$(MAKE) check-admin
	@$(MAKE) build-package
	@for package_dir in admin/packages/core admin/packages/modules/system admin/packages/cli; do \
		(cd "$$package_dir" && CI="$(CI)" $(PNPM) pack --pack-destination "$(NPM_PACK_DEST)") || exit 1; \
	done

# 检查并生成 uni-app npm 包
package-uni-app:
	@$(MAKE) check-uni-app
	@cd uni-app && CI="$(CI)" $(PNPM) run build:packages

# 检查并生成 Taro npm 包
package-taro-app:
	@$(MAKE) check-taro-app
	@cd taro-app && CI="$(CI)" $(PNPM) run build:packages

# 按管理后台、uni-app、Taro 的顺序检查并生成全部 npm 包
package:
	@$(MAKE) package-admin
	@$(MAKE) package-uni-app
	@$(MAKE) package-taro-app
	@echo "==> 全部前端 npm 包生成完成"

# ===== npm 发布 =====

# 检查、打包并发布管理后台 npm 包
publish-admin:
	@$(MAKE) package-admin
	@$(NPM_PUBLISH_ENV) bash scripts/publish_packages.sh admin/packages/core admin/packages/modules/system admin/packages/cli

# 检查、打包并发布 uni-app npm 包
publish-uni-app:
	@$(MAKE) package-uni-app
	@$(NPM_PUBLISH_ENV) bash scripts/publish_packages.sh uni-app/packages/core uni-app/packages/modules/system uni-app/packages/cli

# 检查、打包并发布 Taro npm 包
publish-taro-app:
	@$(MAKE) package-taro-app
	@$(NPM_PUBLISH_ENV) bash scripts/publish_packages.sh taro-app/packages/core taro-app/packages/ui taro-app/packages/modules/system taro-app/packages/cli

# 发布已经检查并构建的全部 npm 包
publish-only:
	@$(NPM_PUBLISH_ENV) bash scripts/publish_packages.sh admin/packages/core admin/packages/modules/system admin/packages/cli uni-app/packages/core uni-app/packages/modules/system uni-app/packages/cli taro-app/packages/core taro-app/packages/ui taro-app/packages/modules/system taro-app/packages/cli

# 按打包、发布的顺序发布全部 npm 包
publish:
	@$(MAKE) package
	@$(MAKE) publish-only

# ===== 帮助 =====

# 查看常用流程、全部目标及可覆盖参数
help:
	@echo ""
	@echo "常用流程:"
	@echo "  make init                         安装三个 workspace 的依赖"
	@echo "  make run                          启动全部前端开发环境（H5 + 微信小程序）"
	@echo "  make check                        检查三个 workspace"
	@echo "  make ts                            生成三个前端的 TypeScript RPC"
	@echo "  make build                        构建管理端、uni-app、Taro 的 H5 + 微信小程序"
	@echo "  make package                      检查并生成 10 个 npm 包"
	@echo "  make publish                      打包并发布 10 个 npm 包"
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
	@printf "  %-24s %s\n" "PNPM" "pnpm 命令，当前: $(PNPM)"
	@printf "  %-24s %s\n" "CI" "是否使用 CI 模式，当前: $(CI)"
	@printf "  %-24s %s\n" "API_DIR" "Backend Proto 目录，当前: $(API_DIR)"
	@printf "  %-24s %s\n" "NPM_REGISTRY" "npm registry，当前: $(NPM_REGISTRY)"
	@printf "  %-24s %s\n" "NPM_ACCESS" "npm 访问级别，当前: $(NPM_ACCESS)"
	@printf "  %-24s %s\n" "NPM_TAG" "npm dist-tag，当前: $(NPM_TAG)"
	@printf "  %-24s %s\n" "NPM_PACK_DEST" "pnpm pack 输出目录，当前: $(NPM_PACK_DEST)"
	@printf "  %-24s %s\n" "NPM_SKIP_GIT_CHECKS" "发布时跳过 pnpm Git 检查，当前: $(NPM_SKIP_GIT_CHECKS)"
	@echo ""
	@echo "示例:"
	@echo "  make run-admin"
	@echo "  make run-h5"
	@echo "  make run-mp-weixin"
	@echo "  make run-uni-app"
	@echo "  make run-uni-app-h5"
	@echo "  make check-taro-app"
	@echo "  make publish NPM_REGISTRY=https://registry.npmjs.org/ NPM_TAG=next"

.DEFAULT_GOAL := help
