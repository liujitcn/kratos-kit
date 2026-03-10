# Not Support Windows

.PHONY: help init plugin cli api tag sub-tag

ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
endif

CURRENT_DIR	:= $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
ROOT_DIR	:= $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

SRCS_MK		:= $(foreach dir, app, $(wildcard $(dir)/*/*/Makefile))


# initialize develop environment
init: plugin cli

# install protoc plugin
plugin:
	# go
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest
	@go install github.com/menta2k/protoc-gen-redact/v3@latest
	@go install github.com/go-kratos/protoc-gen-typescript-http@latest

# install cli tools
cli:
	@go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	@go install github.com/google/gnostic@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install entgo.io/ent/cmd/ent@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# generate protobuf api go code
api:
	@cd api && \
	buf generate

# 根模块：仅根据远程仓库更新状态决定是否打并推送远程 tag（不提交代码）
tag:
	@python3 scripts/tag_release.py tag

# 多模块：递归检查 go.mod 目录，仅根据远程仓库更新状态为模块打并推送远程 tag（不提交代码）
sub-tag:
	@python3 scripts/tag_release.py sub-tag


# show help
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
