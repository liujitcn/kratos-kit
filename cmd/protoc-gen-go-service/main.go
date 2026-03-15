// protoc-gen-go-service 是一个 protoc 插件，用于生成 gRPC service 实现桩代码。
// 需要先将当前程序编译为二进制，并确保它以如下名称出现在 PATH 中：
//
//	protoc-gen-go-service
//
// 这样 protoc 才能通过 `go-service` 后缀识别该插件，并使用如下方式调用：
//
//	protoc --go-service_out=. path/to/service.proto
//
// 生成结果会输出为与 proto 文件同名前缀的 `_service.go` 文件。
//
//	path/to/service_service.go
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

// release is the current protoc-gen-go-grpc version.
const release = "v0.0.1"

var _ *bool

const generatedServicePackage = "service"

// main 解析插件参数并执行 service 代码生成。
func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-go-service %v\n", release)
		return
	}

	var flags flag.FlagSet
	_ = flags.Bool("require_unimplemented_servers", true, "set to false to match legacy behavior")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			// 固定生成到 service 包，避免与 pb.go 共用包名时污染已有实现。
			f.GoPackageName = generatedServicePackage
			generateServiceFile(gen, f)
		}
		return nil
	})
}
