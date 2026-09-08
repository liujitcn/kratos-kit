# 仓库级 Makefile
#
# 常用流程：
#   初始化 Git hooks：make init
#   全仓生成：make gen
#   全仓检查：make check
#   统一打包：make package
#   Docker 镜像：make docker-build

.PHONY: help init hooks check gen \
	build build-backend build-frontend package package-backend package-frontend \
	i18n i18n-check i18n-verify i18n-sync i18n-locale i18n-openapi \
	docker-check docker-config docker-build docker-run docker-stop \
	tag

# ===== 公共路径与参数 =====

BACKEND_DIR ?= backend
FRONTEND_DIR ?= frontend
PYTHON ?= python3
VERSION ?=

# ===== 国际化参数 =====

I18N_LOCALE ?=
I18N_SOURCE_LOCALE ?= zh-CN
I18N_LOCALES ?= en-US,zh-TW,ja-JP
I18N_OFFLINE ?= 0
I18N_AUTO_TRANSLATE ?= 1
I18N_AUTO_LOCALIZE ?= $(I18N_AUTO_TRANSLATE)
I18N_MIGRATION_VERSION ?=
OPENAPI_INPUT ?= backend/internal/openapi/assets/openapi.yaml
OPENAPI_OUTPUT_DIR ?= backend/internal/openapi/assets
OPENAPI_I18N_CONTENT ?= backend/internal/i18n/assets $(wildcard frontend/admin/packages/modules/*/src/locales)

# ===== 统一构建与打包参数 =====

CGO_ENABLED ?= 0
GOOS ?= linux
GOARCH ?= amd64
APP_ENV ?= dev
BUILD_FLAGS ?=
BINARY ?= bin/server
BACKEND_PACKAGE_NAME ?= __PROJECT_NAME__-backend-$(GOOS)-$(GOARCH)
BACKEND_ARCHIVE ?= dist/$(BACKEND_PACKAGE_NAME).tar.gz

# ===== Docker 参数 =====

DOCKER ?= docker
DOCKER_CONTEXT ?= backend
DOCKERFILE ?= backend/Dockerfile
DOCKER_PLATFORM ?= linux/$(GOARCH)
IMAGE ?= __PROJECT_NAME__
TAG ?= latest
DOCKER_BUILD_ARGS ?=
CONTAINER_NAME ?= __PROJECT_NAME__
DOCKER_NETWORK ?= bridge
DOCKER_HTTP_PORT ?= 7001
DOCKER_GRPC_PORT ?= 6001
DOCKER_DATA_DIR ?= backend/data
DOCKER_LOG_DIR ?= backend/logs
DOCKER_BACKUP_DIR ?= backend/backups
DOCKER_CONFIG_SOURCE_DIR ?= backend/configs
DOCKER_CONFIG_DIR ?= backend/runtime/configs
DOCKER_RUN_ARGS ?=

# ===== 环境初始化 =====

# 初始化仓库 Git hooks；具体工具和依赖由 backend/frontend 各自安装。
init:
	@$(MAKE) hooks

# 启用 Git hooks（提交前执行管理端暂存文件检查）。
hooks:
	@chmod +x scripts/githooks/*
	@git config core.hooksPath scripts/githooks
	@echo "==> Git hooks 已启用: scripts/githooks"

# ===== 全仓生成与检查 =====

# 按后端、前端、语言包和 OpenAPI 的顺序生成全仓产物。
gen:
	@$(MAKE) -C "$(BACKEND_DIR)" gen
	@$(MAKE) -C "$(FRONTEND_DIR)" ts
	@$(MAKE) i18n-sync
	@$(MAKE) i18n-openapi
	@echo "==> 全仓代码与文档产物生成完成"

# 按 Backend、Frontend 和国际化一致性的顺序执行全仓检查。
check:
	@$(MAKE) -C "$(BACKEND_DIR)" check
	@$(MAKE) -C "$(FRONTEND_DIR)" check
	@$(MAKE) i18n-check
	@$(MAKE) i18n-verify
	@echo "==> 全仓检查完成"

# ===== 统一构建与打包 =====

# 只构建后端二进制。
build-backend:
	@$(MAKE) -C "$(BACKEND_DIR)" build \
		CGO_ENABLED="$(CGO_ENABLED)" GOOS="$(GOOS)" GOARCH="$(GOARCH)" \
		BUILD_FLAGS="$(BUILD_FLAGS)" BINARY="$(BINARY)"

# 只构建三个前端 H5 宿主；微信小程序产物由 frontend/build-mp-weixin 单独构建。
build-frontend:
	@$(MAKE) -C "$(FRONTEND_DIR)" build-h5

# 构建后端二进制和三个前端 H5 宿主。
build: build-backend build-frontend
	@echo "==> 全仓构建完成"

# 打包后端二进制与运行配置。
package-backend:
	@$(MAKE) -C "$(BACKEND_DIR)" package-binary \
		CGO_ENABLED="$(CGO_ENABLED)" GOOS="$(GOOS)" GOARCH="$(GOARCH)" \
		BUILD_FLAGS="$(BUILD_FLAGS)" BINARY="$(BINARY)" \
		PACKAGE_NAME="$(BACKEND_PACKAGE_NAME)" ARCHIVE="$(BACKEND_ARCHIVE)"

# 打包全部前端 npm 包。
package-frontend:
	@$(MAKE) -C "$(FRONTEND_DIR)" package

# 按后端压缩包、前端 npm 包的顺序完成统一打包。
package: package-backend package-frontend
	@echo "==> 全仓发布包已生成"

# ===== 国际化 =====

# 只检查语言包、语言集合和已提交的注册生成物。
i18n-check:
	@$(PYTHON) scripts/sync_locales.py

# 发布前只读校验语言包、SQL 和 OpenAPI 产物。
i18n-verify:
	@$(PYTHON) scripts/verify_i18n.py \
		--source-locale "$(I18N_SOURCE_LOCALE)" \
		--locales "$(I18N_LOCALES)"

# 同步语言包集合、前端注册文件和代码生成语言目录。
i18n-sync:
	@$(PYTHON) scripts/sync_locales.py --write $(if $(strip $(I18N_MIGRATION_VERSION)),--migration-version "$(I18N_MIGRATION_VERSION)",)

# 生成指定语言的固定语言包和动态翻译 SQL。
i18n-locale:
	@test -n "$(I18N_LOCALE)" || (echo "请指定 I18N_LOCALE，例如 make i18n-locale I18N_LOCALE=ja-JP" && exit 1)
	@$(PYTHON) scripts/generate_locale_drafts.py \
		--write \
		--machine \
		--merge \
		--locale "$(I18N_LOCALE)" \
		$(if $(strip $(I18N_MIGRATION_VERSION)),--migration-version "$(I18N_MIGRATION_VERSION)",) \
		$(if $(filter 1 true,$(I18N_OFFLINE)),--offline,)

# 生成 OpenAPI 源文档和多语言 YAML。
i18n-openapi:
	@$(MAKE) -C "$(BACKEND_DIR)" openapi
	@$(PYTHON) scripts/generate_openapi_locales.py \
		--input "$(OPENAPI_INPUT)" \
		--output-dir "$(OPENAPI_OUTPUT_DIR)" \
		--source-locale "$(I18N_SOURCE_LOCALE)" \
		--locale "$(I18N_LOCALES)" \
		$(foreach content,$(OPENAPI_I18N_CONTENT),--i18n-content "$(content)") \
		$(if $(filter 1 true,$(I18N_AUTO_LOCALIZE)),--auto-i18n,) \
		$(if $(filter 1 true,$(I18N_OFFLINE)),--offline,)
	@find "$(OPENAPI_OUTPUT_DIR)" -type f -name '*.yaml' -exec perl -pi -e 's/BaseI18N/BaseI18n/g; s/baseI18N/baseI18n/g' {} +
	@echo "==> OpenAPI v3 多语言文档生成完成"

# 按语言包、OpenAPI 的顺序执行常规国际化生成。
i18n:
	@$(MAKE) i18n-sync
	@$(MAKE) i18n-openapi
	@echo "==> 全仓国际化产物生成完成"

# ===== Docker =====

# 检查 Docker 命令和服务端是否可用。
docker-check:
	@command -v "$(DOCKER)" >/dev/null 2>&1 || (echo "未找到 Docker 命令: $(DOCKER)" && exit 1)
	@"$(DOCKER)" info >/dev/null 2>&1 || (echo "Docker 服务不可用，请确认 Docker Desktop 或 Docker daemon 已启动" && exit 1)
	@echo "==> Docker 可用: $$($(DOCKER) version --format 'client={{.Client.Version}} server={{.Server.Version}}')"

# 初始化可由宿主机修改的容器运行配置。
docker-config:
	@test -d "$(DOCKER_CONFIG_SOURCE_DIR)" || (echo "未找到源配置目录: $(DOCKER_CONFIG_SOURCE_DIR)" && exit 1)
	@if [ ! -d "$(DOCKER_CONFIG_DIR)" ]; then \
		mkdir -p "$(DOCKER_CONFIG_DIR)"; \
		cp "$(DOCKER_CONFIG_SOURCE_DIR)"/*.yaml "$(DOCKER_CONFIG_DIR)/"; \
		for config_file in data.yaml data.dev.yaml logger.yaml pprof.yaml registry.yaml key.yaml; do \
			if [ -f "$(DOCKER_CONFIG_DIR)/$$config_file" ]; then \
				perl -pi -e 's/127\.0\.0\.1/host.docker.internal/g; s/localhost/host.docker.internal/g' "$(DOCKER_CONFIG_DIR)/$$config_file"; \
			fi; \
		done; \
		echo "==> Docker 运行配置已初始化: $(DOCKER_CONFIG_DIR)"; \
	fi

# 构建三端静态资源、后端程序和 Docker 镜像。
docker-build: docker-check
	@test -f "$(DOCKERFILE)" || (echo "未找到 Dockerfile: $(DOCKERFILE)，请通过 DOCKERFILE 指定有效文件" && exit 1)
	@$(MAKE) build-frontend
	@$(MAKE) -C "$(BACKEND_DIR)" build \
		CGO_ENABLED="$(CGO_ENABLED)" GOOS=linux GOARCH="$(GOARCH)" \
		BUILD_FLAGS="$(BUILD_FLAGS)" BINARY="$(BINARY)"
	@"$(DOCKER)" build $(DOCKER_BUILD_ARGS) --platform "$(DOCKER_PLATFORM)" -f "$(DOCKERFILE)" -t "$(IMAGE):$(TAG)" "$(DOCKER_CONTEXT)"
	@echo "==> Docker 镜像已生成: $(IMAGE):$(TAG)"

# 使用宿主机数据和配置目录启动容器。
docker-run: docker-check docker-config
	@"$(DOCKER)" image inspect "$(IMAGE):$(TAG)" >/dev/null 2>&1 || (echo "未找到 Docker 镜像: $(IMAGE):$(TAG)，请先执行 make docker-build" && exit 1)
	@test -d "$(DOCKER_CONFIG_DIR)" || (echo "未找到宿主机配置目录: $(DOCKER_CONFIG_DIR)" && exit 1)
	@mkdir -p "$(DOCKER_DATA_DIR)"
	@mkdir -p "$(DOCKER_LOG_DIR)" "$(DOCKER_BACKUP_DIR)"
	@if "$(DOCKER)" container inspect "$(CONTAINER_NAME)" >/dev/null 2>&1; then \
		container_status=$$("$(DOCKER)" container inspect -f '{{.State.Status}}' "$(CONTAINER_NAME)"); \
		if [ "$$container_status" = "running" ]; then \
			echo "容器正在运行: $(CONTAINER_NAME)，请先执行 make docker-stop"; \
			exit 1; \
		fi; \
		"$(DOCKER)" container rm "$(CONTAINER_NAME)" >/dev/null; \
		echo "==> 已移除已停止容器: $(CONTAINER_NAME)"; \
	fi
	@"$(DOCKER)" run -d \
		--name "$(CONTAINER_NAME)" \
		--restart unless-stopped \
		--network "$(DOCKER_NETWORK)" \
		--add-host "host.docker.internal:host-gateway" \
		-e APP_ENV="$(APP_ENV)" \
		-e VAULT_TOKEN \
		-p "$(DOCKER_HTTP_PORT):7001" \
		-p "$(DOCKER_GRPC_PORT):6001" \
		-v "$(abspath $(DOCKER_DATA_DIR)):/app/data" \
		-v "$(abspath $(DOCKER_LOG_DIR)):/app/logs" \
		-v "$(abspath $(DOCKER_BACKUP_DIR)):/app/backups" \
		-v "$(abspath $(DOCKER_CONFIG_DIR)):/app/configs:ro" \
		$(DOCKER_RUN_ARGS) \
		"$(IMAGE):$(TAG)" \
		./server -c ./configs -e "$(APP_ENV)"
	@echo "==> Docker 容器已启动: $(CONTAINER_NAME)"
	@echo "==> HTTP: http://127.0.0.1:$(DOCKER_HTTP_PORT)"
	@echo "==> gRPC: 127.0.0.1:$(DOCKER_GRPC_PORT)"

# 停止本地 Docker 容器并保留容器数据。
docker-stop: docker-check
	@if ! "$(DOCKER)" container inspect "$(CONTAINER_NAME)" >/dev/null 2>&1; then \
		echo "容器不存在: $(CONTAINER_NAME)"; \
	elif [ "$$($(DOCKER) container inspect -f '{{.State.Status}}' "$(CONTAINER_NAME)")" != "running" ]; then \
		echo "容器已停止: $(CONTAINER_NAME)"; \
	else \
		"$(DOCKER)" stop "$(CONTAINER_NAME)"; \
		echo "==> Docker 容器已停止: $(CONTAINER_NAME)"; \
	fi

# ===== 发布 =====

# 统一升级、提交、打包并发布 Backend 与前端 npm 包。
tag:
	@$(MAKE) i18n-verify
	@$(PYTHON) scripts/tag_release.py $(if $(strip $(VERSION)),--version "$(VERSION)",)

# ===== 帮助 =====

# 查看仓库级目标及可覆盖参数。
help:
	@echo ""
	@echo "常用流程:"
	@echo "  make init                         初始化 Git hooks"
	@echo "  make gen                          生成后端、前端和文档产物"
	@echo "  make check                        执行 Backend、Frontend 和国际化检查"
	@echo "  make build                        构建后端和三个前端宿主"
	@echo "  make package                      生成后端压缩包和全部 npm 包"
	@echo "  make docker-build                 构建 Docker 镜像"
	@echo "  make tag VERSION=0.0.1            统一发布（会提交并推送）"
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
	@printf "  %-24s %s\n" "VERSION" "发布版本；为空时由发布脚本决定"
	@printf "  %-24s %s\n" "GOOS / GOARCH" "后端和镜像目标平台，当前: $(GOOS) / $(GOARCH)"
	@printf "  %-24s %s\n" "APP_ENV" "Docker 容器运行环境，当前: $(APP_ENV)"
	@printf "  %-24s %s\n" "BINARY" "后端二进制输出，当前: $(BINARY)"
	@printf "  %-24s %s\n" "BACKEND_ARCHIVE" "后端目录内压缩包输出，当前: $(BACKEND_ARCHIVE)"
	@printf "  %-24s %s\n" "I18N_LOCALES" "目标语言，当前: $(I18N_LOCALES)"
	@printf "  %-24s %s\n" "I18N_SOURCE_LOCALE" "源语言，当前: $(I18N_SOURCE_LOCALE)"
	@printf "  %-24s %s\n" "I18N_OFFLINE" "设为 1 时离线生成"
	@printf "  %-24s %s\n" "I18N_AUTO_LOCALIZE" "OpenAPI 是否自动翻译，当前: $(I18N_AUTO_LOCALIZE)"
	@printf "  %-24s %s\n" "IMAGE / TAG" "Docker 镜像，当前: $(IMAGE):$(TAG)"
	@printf "  %-24s %s\n" "DOCKER_CONTEXT" "Docker 构建上下文，当前: $(DOCKER_CONTEXT)"
	@printf "  %-24s %s\n" "DOCKERFILE" "Dockerfile 路径，当前: $(DOCKERFILE)"
	@printf "  %-24s %s\n" "DOCKER_PLATFORM" "Docker 平台，当前: $(DOCKER_PLATFORM)"
	@printf "  %-24s %s\n" "CONTAINER_NAME" "Docker 容器名，当前: $(CONTAINER_NAME)"
	@printf "  %-24s %s\n" "DOCKER_NETWORK" "Docker 网络，当前: $(DOCKER_NETWORK)"
	@printf "  %-24s %s\n" "DOCKER_HTTP_PORT" "宿主机 HTTP 端口，当前: $(DOCKER_HTTP_PORT)"
	@printf "  %-24s %s\n" "DOCKER_GRPC_PORT" "宿主机 gRPC 端口，当前: $(DOCKER_GRPC_PORT)"
	@printf "  %-24s %s\n" "DOCKER_DATA_DIR" "宿主机数据目录，当前: $(DOCKER_DATA_DIR)"
	@printf "  %-24s %s\n" "DOCKER_LOG_DIR" "宿主机日志目录，当前: $(DOCKER_LOG_DIR)"
	@printf "  %-24s %s\n" "DOCKER_BACKUP_DIR" "宿主机备份目录，当前: $(DOCKER_BACKUP_DIR)"
	@printf "  %-24s %s\n" "DOCKER_CONFIG_SOURCE_DIR" "Docker 配置源目录，当前: $(DOCKER_CONFIG_SOURCE_DIR)"
	@printf "  %-24s %s\n" "DOCKER_CONFIG_DIR" "宿主机配置目录，当前: $(DOCKER_CONFIG_DIR)"
	@echo ""
	@echo "更多命令:"
	@echo "  make -C backend help              查看 Backend 命令"
	@echo "  make -C frontend help             查看 Frontend 命令"

.DEFAULT_GOAL := help
