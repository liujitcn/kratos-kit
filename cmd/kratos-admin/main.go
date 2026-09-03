// Command kratos-admin 创建包含前后端的完整项目。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const commandName = "kratos-admin"

// main 解析命令行并执行后端项目生成。
func main() {
	var err error
	err = run(os.Args[1:], os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 解析 create 命令并生成完整前后端项目。
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
	if len(args) < 2 {
		return fmt.Errorf("%s", usageText())
	}

	flags := flag.NewFlagSet(commandName+" create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var projectName string
	projectName = args[1]
	var modulePath string
	flags.StringVar(&modulePath, "module", "", "后端项目的 Go module，默认 github.com/example/<project>/backend")
	var frontendModule string
	flags.StringVar(&frontendModule, "frontend-module", "app", "前端默认业务 module 名称")
	var err error
	err = flags.Parse(args[2:])
	if err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("create 只接受项目名、--module 和 --frontend-module 参数")
	}

	var cwd string
	cwd, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录: %w", err)
	}
	var target string
	target, err = createProjectWithOptions(
		projectOptions{
			projectName:    projectName,
			modulePath:     modulePath,
			frontendModule: frontendModule,
		},
		cwd,
		initializeProject,
	)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "已创建完整项目: %s\n", target)
	return err
}

// printHelp 输出命令帮助。
func printHelp(output io.Writer) {
	_, _ = fmt.Fprintf(
		output,
		"%s\n\n用法:\n  %s create <project> [--module <go-module>] [--frontend-module <module>]\n\n示例:\n  %s create shop-admin\n  %s create shop-admin --module github.com/acme/shop-admin/backend --frontend-module shop\n",
		"创建包含前后端和 Admin 能力的完整项目，前端通过管理端、uni-app 和 Taro CLI 生成。",
		commandName,
		commandName,
		commandName,
	)
}

// usageText 返回命令行参数错误时使用的简短用法。
func usageText() string {
	return "用法: " + commandName + " create <project> [--module <go-module>] [--frontend-module <module>]"
}
