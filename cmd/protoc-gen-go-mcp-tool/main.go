// protoc-gen-go-mcp-tool 是一个 protoc 插件，用于生成 MCP 可直接调用 service 方法的 Tool 注册代码。
// 需要先将当前程序编译为二进制，并确保它以如下名称出现在 PATH 中：
//
//	protoc-gen-go-mcp-tool
//
// 这样 protoc 才能通过 `go-mcp-tool` 后缀识别该插件，并使用如下方式调用：
//
//	protoc --go-mcp-tool_out=. path/to/service.proto
//
// 生成结果会输出为与 proto 文件同名前缀的 `_mcp_tool.go` 文件。
//
//	path/to/service_mcp_tool.go
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const release = "v0.0.1"

// main 解析插件参数并执行 MCP Tool 代码生成。
func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-go-mcp-tool %v\n", release)
		return
	}

	var flags flag.FlagSet
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, file := range gen.Files {
			if !file.Generate {
				continue
			}
			generateMCPToolFile(gen, file)
		}
		return nil
	})
}
