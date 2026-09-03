# Makefile 提供完整项目的前后端聚合命令。
.PHONY: help init backend-init frontend-install generate test build run backend-run infra-up infra-down \
	frontend-dev frontend-admin-dev frontend-uni-dev frontend-taro-dev \
	frontend-build frontend-admin-build frontend-uni-build frontend-taro-build

# 初始化后端工具和前端依赖。
init: backend-init frontend-install

# 初始化后端生成工具。
backend-init:
	@$(MAKE) -C backend init

# 安装前端 workspace 依赖。
frontend-install:
	@$(MAKE) -C frontend init

# 生成后端协议、OpenAPI 和 Wire 产物。
generate:
	@$(MAKE) -C backend gen

# 运行后端测试。
test:
	@$(MAKE) -C backend test

# 构建后端和前端生产产物。
build:
	@$(MAKE) -C backend build
	@$(MAKE) frontend-build

# 启动后端服务。
run:
	@$(MAKE) -C backend run

# 只启动后端服务。
backend-run:
	@bash scripts/backend.sh run

# 启动 Admin profile 提供的本地基础设施。
infra-up:
	@test -f docker-compose.yaml || (echo "当前 profile 未提供 docker-compose.yaml" && exit 1)
	@docker compose up -d

# 停止 Admin profile 提供的本地基础设施。
infra-down:
	@test -f docker-compose.yaml || (echo "当前 profile 未提供 docker-compose.yaml" && exit 1)
	@docker compose down

# 启动前端开发服务。
frontend-dev:
	@$(MAKE) frontend-admin-dev

# 启动管理端前端开发服务。
frontend-admin-dev:
	@bash scripts/frontend-admin.sh dev

# 启动 uni-app H5 开发服务。
frontend-uni-dev:
	@bash scripts/frontend-uni-app.sh dev:h5

# 启动 Taro H5 开发服务。
frontend-taro-dev:
	@bash scripts/frontend-taro-app.sh dev:h5

# 构建三套前端 H5 生产产物。
frontend-build: frontend-admin-build frontend-uni-build frontend-taro-build

# 构建管理端前端。
frontend-admin-build:
	@bash scripts/frontend-admin.sh build

# 构建 uni-app H5 产物。
frontend-uni-build:
	@bash scripts/frontend-uni-app.sh build:h5

# 构建 Taro H5 产物。
frontend-taro-build:
	@bash scripts/frontend-taro-app.sh build:h5

# 显示帮助。
help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets: init generate test build run backend-run infra-up infra-down frontend-dev frontend-uni-dev frontend-taro-dev"

.DEFAULT_GOAL := help
