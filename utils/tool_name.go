package utils

import (
	"strings"

	"github.com/liujitcn/go-utils/stringcase"
)

// ToolNameFromRPCPath 根据标准 RPC 路径生成 Tool 名称。
func ToolNameFromRPCPath(rpcPath string) string {
	toolName := strings.Trim(strings.TrimSpace(rpcPath), "/")
	toolName = strings.ReplaceAll(toolName, "/", "_")
	toolName = strings.ReplaceAll(toolName, ".", "_")
	return stringcase.ToSnakeCase(toolName)
}
