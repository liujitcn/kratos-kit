package utils

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// FormatGeneratedContent 使用 gofmt 规则整理生成代码。
func FormatGeneratedContent(filename string, content []byte) ([]byte, error) {
	formatted, err := format.Source(content)
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

// StripRedundantImportAliases 去掉冗余 import 别名，并修正版本目录包别名。
func StripRedundantImportAliases(content []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			importPath := strings.Trim(importSpec.Path.Value, "\"")
			if importSpec.Name == nil && IsVersionedImportPath(importPath) {
				// Go 语法不会为 v1/v2 这类版本目录自动推断真实包名，生成代码必须补齐别名。
				importSpec.Name = ast.NewIdent(DefaultVersionedImportAlias(importPath))
				continue
			}
			if importSpec.Name != nil && importSpec.Name.Name == "_" && IsVersionedImportPath(importPath) {
				// 手写 appv1.Xxx 引用后，protogen.Import 可能先生成空白导入，这里改回真实包别名。
				importSpec.Name = ast.NewIdent(DefaultVersionedImportAlias(importPath))
				continue
			}
			if importSpec.Name == nil {
				continue
			}
			alias := importSpec.Name.Name
			if alias == path.Base(importPath) && IsVersionedImportPath(importPath) {
				// protogen 可能先生成 v1 这类别名，这里统一改成 appv1/adminv1 等真实包名。
				importSpec.Name = ast.NewIdent(DefaultVersionedImportAlias(importPath))
				continue
			}
			if !isRedundantImportAlias(alias, importPath) || IsVersionedImportPath(importPath) {
				continue
			}
			// 包名与 import path 默认名一致时，直接省略别名。
			importSpec.Name = nil
		}
	}

	// 统一按 gofmt 期望的顺序整理 import，避免生成结果顺序漂移。
	ast.SortImports(fset, file)

	var buf bytes.Buffer
	if err = format.Node(&buf, fset, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NormalizeCommentText 提取 proto 注释文本，并清理多余的注释标记与空白。
func NormalizeCommentText(comments protogen.Comments) string {
	raw := strings.TrimSpace(string(comments))
	if raw == "" {
		return ""
	}

	lines := strings.Split(raw, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

// FormatToolActionComment 生成 Tool 构造或注册方法注释。
func FormatToolActionComment(name, comment, action, toolKind string) string {
	if comment == "" {
		return "// " + name
	}
	return "// " + name + " " + action + trimTrailingChinesePunctuation(comment) + "的 " + toolKind + "。"
}

// ToolDescription 生成 Tool 描述，优先使用 RPC 注释，其次使用 service 注释和方法名兜底。
func ToolDescription(service *protogen.Service, method *protogen.Method) string {
	methodComment := NormalizeCommentText(method.Comments.Leading)
	if methodComment != "" {
		return methodComment
	}
	serviceComment := NormalizeCommentText(service.Comments.Leading)
	if serviceComment != "" {
		return serviceComment + " - " + method.GoName
	}
	return service.GoName + "." + method.GoName
}

// IsUnaryMethod 判断当前 RPC 是否为普通一元调用。
func IsUnaryMethod(method *protogen.Method) bool {
	return !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer()
}

// ProtocVersion 返回当前 protoc 编译器版本信息。
func ProtocVersion(gen *protogen.Plugin) string {
	v := gen.Request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}
	var suffix string
	if value := v.GetSuffix(); value != "" {
		suffix = "-" + value
	}
	return fmt.Sprintf("v%d.%d.%d%s", v.GetMajor(), v.GetMinor(), v.GetPatch(), suffix)
}

// Unexport 将首字母转换为小写。
func Unexport(source string) string {
	if source == "" {
		return ""
	}
	return strings.ToLower(source[:1]) + source[1:]
}

// IsVersionedImportPath 判断 import path 最后一段是否为 v1/v2 这类版本目录。
func IsVersionedImportPath(importPath string) bool {
	return IsVersionSegment(path.Base(importPath))
}

// IsVersionSegment 判断路径段是否为 v1/v2 这类 proto 版本目录。
func IsVersionSegment(source string) bool {
	if len(source) < 2 || source[0] != 'v' {
		return false
	}
	for _, value := range source[1:] {
		if !isDigitASCII(value) {
			return false
		}
	}
	return true
}

// DefaultVersionedImportAlias 返回版本目录常用的生成包别名，例如 common/v1 -> commonv1。
func DefaultVersionedImportAlias(importPath string) string {
	dir := path.Base(path.Dir(importPath))
	version := path.Base(importPath)
	return sanitizeImportAlias(dir) + version
}

// trimTrailingChinesePunctuation 清理注释末尾标点，方便拼接生成方法注释。
func trimTrailingChinesePunctuation(source string) string {
	return strings.TrimRight(source, "。.!！?？；;")
}

// sanitizeImportAlias 清理 import 别名中的非法字符。
func sanitizeImportAlias(source string) string {
	var builder strings.Builder
	for _, value := range source {
		if isLowerASCII(value) || isUpperASCII(value) || isDigitASCII(value) {
			builder.WriteRune(value)
		}
	}
	if builder.Len() == 0 {
		return "pb"
	}
	return builder.String()
}

// isRedundantImportAlias 判断 import 别名是否仅重复了默认包名。
func isRedundantImportAlias(alias, importPath string) bool {
	return path.Base(importPath) == alias
}

// isUpperASCII 判断字符是否为 ASCII 大写字母。
func isUpperASCII(value rune) bool {
	return value >= 'A' && value <= 'Z'
}

// isLowerASCII 判断字符是否为 ASCII 小写字母。
func isLowerASCII(value rune) bool {
	return value >= 'a' && value <= 'z'
}

// isDigitASCII 判断字符是否为 ASCII 数字。
func isDigitASCII(value rune) bool {
	return value >= '0' && value <= '9'
}
