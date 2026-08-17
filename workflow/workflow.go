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
