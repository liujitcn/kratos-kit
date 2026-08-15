package cron

import "context"

// TaskExec 定义带上下文的数据库任务执行方法。
type TaskExec interface {
	// Exec 执行任务并返回输出内容。
	Exec(context.Context, map[string]string) ([]string, error)
}

// Task 描述一项按名称调用的数据库任务执行器。
type Task struct {
	// Name 是任务名称，也是数据库任务的 invoke_target。
	Name string
	// Exec 是任务执行器。
	Exec TaskExec
}
