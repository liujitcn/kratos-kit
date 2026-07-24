// Package workflow 定义工作流引擎插件的公共接口。
//
// 每个引擎插件（argo、conductor、goworkflows、temporal）都会封装对应的原生
// SDK 客户端，并实现 Client 接口。启动、取消、发送信号等无法统一抽象的能力，
// 由各具体客户端类型自行暴露。
package workflow

// Client 是所有工作流引擎客户端的公共接口。
// 各引擎只有关闭资源这一项能力完全一致，其他能力由具体类型提供。
type Client interface {
	// Close 释放客户端连接与相关资源。
	Close() error
}

// Worker 是工作流引擎 Worker 的公共接口。
// conductor、goworkflows、temporal 插件会实现该接口，Argo 没有 Worker 概念。
type Worker interface {
	// Stop 通知 Worker 停止轮询任务。
	Stop()
	// IsRunning 返回 Worker 是否仍处于运行状态。
	IsRunning() bool
}
