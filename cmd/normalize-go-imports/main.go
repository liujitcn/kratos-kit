// Command normalize-go-imports normalizes explicit Go import aliases.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config 配置 Go import 别名规范化范围和写入行为。
type Config struct {
	// Root 是仓库或工作区根目录，默认为当前目录。
	Root string
	// Modules 是相对于 Root 的 Go module 目录；为空时自动发现所有 go.mod。
	Modules []string
	// Files 是待处理文件；为空时递归处理 Root 下的 Go 文件。
	Files []string
	// Write 为 true 时将结果写回文件，否则只返回变更。
	Write bool
}

// Change 描述一个 import 别名变更。
type Change struct {
	File       string
	ImportPath string
	From       string
	To         string
}

type stringList []string

// String 返回命令行参数列表的逗号分隔表示。
func (s *stringList) String() string { return strings.Join(*s, ",") }

// Set 追加一个命令行参数值。
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type edit struct {
	start int
	end   int
	text  string
}

type importInfo struct {
	spec    *ast.ImportSpec
	path    string
	actual  string
	oldName string
	newName string
}

func main() {
	if err := Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Run 解析命令行参数并执行 import 别名规范化。
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("normalize-go-imports", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository or workspace root")
	var modules stringList
	flags.Var(&modules, "module", "module directory relative to root; may be repeated")
	var files stringList
	flags.Var(&files, "file", "Go file relative to root; may be repeated")
	write := flags.Bool("write", false, "write normalized files")
	if err := flags.Parse(args); err != nil {
		return err
	}

	changes, err := Normalize(Config{
		Root:    *root,
		Modules: modules,
		Files:   files,
		Write:   *write,
	})
	if err != nil {
		return err
	}
	for _, change := range changes {
		_, err = fmt.Fprintf(stdout, "%s\n  %s: %s -> %s\n", change.File, change.ImportPath, change.From, change.To)
		if err != nil {
			return fmt.Errorf("write changes: %w", err)
		}
	}
	return nil
}

// Normalize 批量规范化 Go 文件中的显式 import 别名。
//
// 该能力只负责别名规范化，不负责补充、删除或排序 import；这些职责应交给
// goimports。Write 为 false 时不会修改任何文件。
func Normalize(config Config) ([]Change, error) {
	root := config.Root
	if root == "" {
		root = "."
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	var errs []error

	modules := config.Modules
	if len(modules) == 0 {
		modules, err = discoverModules(root)
		if err != nil {
			return nil, err
		}
	}
	var packages map[string]string
	packages, err = LoadPackages(root, modules)
	if err != nil {
		if len(packages) == 0 {
			return nil, err
		}
		errs = append(errs, err)
	}

	files := config.Files
	if len(files) == 0 {
		files, err = collectGoFiles(root)
		if err != nil {
			return nil, err
		}
	} else {
		files = resolveFiles(root, files)
	}

	changes := make([]Change, 0)
	for _, filename := range files {
		var fileChanges []Change
		fileChanges, err = normalizeFile(filename, packages, config.Write)
		changes = append(changes, fileChanges...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return changes, errors.Join(errs...)
}

// LoadPackages 加载 module 依赖的 import path 与实际包名映射。
func LoadPackages(root string, modules []string) (map[string]string, error) {
	packages := make(map[string]string)
	var errs []error
	var err error
	for _, module := range modules {
		moduleDir := module
		if !filepath.IsAbs(moduleDir) {
			moduleDir = filepath.Join(root, moduleDir)
		}
		moduleDir, err = filepath.Abs(moduleDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("resolve module %q: %w", module, err))
			continue
		}

		command := exec.Command("go", "list", "-e", "-deps", "-f", "{{.ImportPath}}{{\"\\t\"}}{{.Name}}", "./...")
		command.Dir = moduleDir
		var output []byte
		output, err = command.Output()
		if err != nil {
			errs = append(errs, fmt.Errorf("go list %s: %w", moduleDir, err))
		}
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			fields := strings.SplitN(scanner.Text(), "\t", 2)
			if len(fields) == 2 && fields[0] != "" && fields[1] != "" {
				packages[fields[0]] = fields[1]
			}
		}
		err = scanner.Err()
		if err != nil {
			errs = append(errs, fmt.Errorf("read go list %s: %w", moduleDir, err))
		}
	}
	return packages, errors.Join(errs...)
}

// NormalizeSource 规范化一份 Go 源码并返回新源码与变更记录。
//
// filename 仅用于错误信息和变更记录，不要求文件真实存在。packages 应由
// LoadPackages 提供，键为 import path，值为依赖包声明的实际 package name。
func NormalizeSource(filename string, source []byte, packages map[string]string) ([]byte, []Change, error) {
	fset := token.NewFileSet()
	var file *ast.File
	var err error
	file, err = parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	var imports []importInfo
	var implicitNames map[string]bool
	imports, implicitNames, err = collectImports(file, packages)
	if err != nil {
		return nil, nil, fmt.Errorf("collect imports %s: %w", filename, err)
	}
	if len(imports) == 0 {
		return append([]byte(nil), source...), nil, nil
	}

	groups := make(map[string][]int)
	for i := range imports {
		groups[imports[i].actual] = append(groups[imports[i].actual], i)
	}
	localNames := collectLocalNames(file)
	currentNames := make(map[string]bool, len(implicitNames)+len(imports))
	for name := range implicitNames {
		currentNames[name] = true
	}
	for i := range imports {
		currentNames[imports[i].oldName] = true
	}

	for actual, indexes := range groups {
		if !canUse(actual, implicitNames, localNames) {
			continue
		}
		if len(indexes) == 1 {
			index := indexes[0]
			// 局部声明可能遮蔽旧别名；此时无法仅靠 AST 名称安全改写选择器。
			if localNames[imports[index].oldName] || (currentNames[actual] && imports[index].oldName != actual) {
				continue
			}
			imports[index].newName = actual
			continue
		}

		chosen := indexes[0]
		for _, index := range indexes {
			if imports[index].oldName == actual {
				chosen = index
				break
			}
		}
		if localNames[imports[chosen].oldName] || (currentNames[actual] && imports[chosen].oldName != actual) {
			continue
		}
		imports[chosen].newName = actual
	}
	for i := range imports {
		if imports[i].newName == "" {
			imports[i].newName = imports[i].oldName
		}
	}
	ensureUniqueNames(imports)

	edits := make([]edit, 0)
	changes := make([]Change, 0)
	for _, info := range imports {
		if info.newName == info.oldName {
			continue
		}
		if info.newName == info.actual {
			nameStart := offset(fset, info.spec.Name.Pos())
			pathStart := offset(fset, info.spec.Path.Pos())
			edits = append(edits, edit{start: nameStart, end: pathStart})
		} else {
			nameStart := offset(fset, info.spec.Name.Pos())
			nameEnd := offset(fset, info.spec.Name.End())
			edits = append(edits, edit{start: nameStart, end: nameEnd, text: info.newName})
		}
		changes = append(changes, Change{
			File:       filename,
			ImportPath: info.path,
			From:       info.oldName,
			To:         info.newName,
		})
		oldName, newName := info.oldName, info.newName
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == oldName {
				start := offset(fset, ident.Pos())
				end := offset(fset, ident.End())
				edits = append(edits, edit{start: start, end: end, text: newName})
			}
			return true
		})
	}
	if len(edits) == 0 {
		return append([]byte(nil), source...), nil, nil
	}

	var updated []byte
	updated, err = applyEdits(source, edits)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite %s: %w", filename, err)
	}
	return updated, changes, nil
}

// NormalizeFile 读取文件并返回规范化结果，不会写回文件。
func NormalizeFile(filename string, packages map[string]string) ([]byte, []Change, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", filename, err)
	}
	return NormalizeSource(filename, source, packages)
}

func normalizeFile(filename string, packages map[string]string, write bool) ([]Change, error) {
	updated, changes, err := NormalizeFile(filename, packages)
	if err != nil {
		return nil, err
	}
	if write && len(changes) > 0 {
		if err = os.WriteFile(filename, updated, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return changes, nil
}

func discoverModules(root string) ([]string, error) {
	modules := make([]string, 0)
	var err error
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		var relative string
		relative, err = filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		if relative == "." {
			relative = "."
		}
		modules = append(modules, relative)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Go modules: %w", err)
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no go.mod found below %s; pass -module explicitly", root)
	}
	sort.Strings(modules)
	return modules, nil
}

func collectGoFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect Go files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func resolveFiles(root string, files []string) []string {
	resolved := make([]string, 0, len(files))
	for _, filename := range files {
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(root, filename)
		}
		resolved = append(resolved, filepath.Clean(filename))
	}
	sort.Strings(resolved)
	return resolved
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "testdata":
		return true
	default:
		return false
	}
}

func collectImports(file *ast.File, packages map[string]string) ([]importInfo, map[string]bool, error) {
	imports := make([]importInfo, 0, len(file.Imports))
	implicitNames := make(map[string]bool)
	var err error
	for _, spec := range file.Imports {
		var path string
		path, err = strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, nil, err
		}
		actual := packages[path]
		if actual == "" {
			continue
		}
		if spec.Name == nil {
			implicitNames[actual] = true
			continue
		}
		if spec.Name.Name == "." || spec.Name.Name == "_" {
			continue
		}
		imports = append(imports, importInfo{spec: spec, path: path, actual: actual, oldName: spec.Name.Name})
	}
	return imports, implicitNames, nil
}

func collectLocalNames(file *ast.File) map[string]bool {
	names := make(map[string]bool)
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			collectFunctionNames(function.Type, names)
			collectFunctionNames(function.Body, names)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncLit)
		if ok {
			collectFunctionNames(function.Type, names)
			collectFunctionNames(function.Body, names)
		}
		return true
	})
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Name != nil {
				names[declaration.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					names[spec.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						names[name.Name] = true
					}
				}
			}
		}
	}
	return names
}

func collectFunctionNames(node ast.Node, names map[string]bool) {
	ast.Inspect(node, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && ident.Obj != nil {
			names[ident.Name] = true
		}
		return true
	})
}

func canUse(actual string, implicitNames, localNames map[string]bool) bool {
	return token.IsIdentifier(actual) && !token.Lookup(actual).IsKeyword() && !localNames[actual] && !implicitNames[actual]
}

func ensureUniqueNames(imports []importInfo) {
	used := make(map[string]bool)
	for i := range imports {
		name := imports[i].newName
		if !used[name] {
			used[name] = true
			continue
		}
		imports[i].newName = imports[i].oldName
		used[imports[i].newName] = true
	}
}

func applyEdits(source []byte, edits []edit) ([]byte, error) {
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start == edits[j].start {
			return edits[i].end > edits[j].end
		}
		return edits[i].start > edits[j].start
	})
	updated := append([]byte(nil), source...)
	previousStart := len(source) + 1
	for _, item := range edits {
		if item.start < 0 || item.end < item.start || item.end > len(updated) || item.end > previousStart {
			return nil, fmt.Errorf("overlapping or invalid edit [%d,%d)", item.start, item.end)
		}
		updated = append(updated[:item.start], append([]byte(item.text), updated[item.end:]...)...)
		previousStart = item.start
	}
	return updated, nil
}

func offset(fset *token.FileSet, pos token.Pos) int {
	return fset.File(pos).Offset(pos)
}
