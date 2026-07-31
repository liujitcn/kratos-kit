// Command kratos-admin-backend 创建基于 kratos-admin Core 的后端项目。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const commandName = "kratos-admin-backend"

// main 解析命令行并执行后端项目生成。
func main() {
	var err error
	err = run(os.Args[1:], os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 解析 create 命令并生成后端项目。
func run(args []string, output io.Writer) error {
	if len(args) == 0 ||
		args[0] == "help" ||
		args[0] == "--help" ||
		args[0] == "-h" ||
		(len(args) == 2 && args[0] == "create" && (args[1] == "--help" || args[1] == "-h")) {
		printHelp(output)
		return nil
	}
	if args[0] != "create" {
		return fmt.Errorf("不支持的命令: %s", args[0])
	}

	flags := flag.NewFlagSet(commandName+" create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var modulePath string
	flags.StringVar(&modulePath, "module", "", "生成项目的 Go module")
	var err error
	err = flags.Parse(args[1:])
	if err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("create 只接受 --module 参数")
	}

	var cwd string
	cwd, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录: %w", err)
	}
	var target string
	target, err = createProject(modulePath, cwd)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "已创建后端项目: %s\n", target)
	return err
}

// printHelp 输出命令帮助。
func printHelp(output io.Writer) {
	_, _ = fmt.Fprintf(
		output,
		"%s\n\n用法:\n  %s create --module <go-module>\n\n示例:\n  %s create --module github.com/example/order\n",
		"创建基于 github.com/liujitcn/kratos-admin/backend/core 的后端项目。",
		commandName,
		commandName,
	)
}
