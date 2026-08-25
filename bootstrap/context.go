package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	kratosRegistry "github.com/go-kratos/kratos/v3/registry"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

// Context 引导上下文
type Context struct {
	config        *configv1.Bootstrap // 引导配置
	appInfo       *configv1.AppInfo   // 应用信息
	appInfoConfig *configv1.AppInfo   // 调用方传入的原始应用信息

	logger    *slog.Logger             // 日志记录器
	registrar kratosRegistry.Registrar // 服务注册器

	customConfig sync.Map // 自定义配置项
	values       sync.Map // 自定义值存储

	rootCtx context.Context    // 应用级根上下文（可用于优雅关闭）
	cancel  context.CancelFunc // 取消函数
}

// NewContext 创建带 cancel 的应用级 Context（传 nil 使用 Background）
func NewContext(parent context.Context, ai *configv1.AppInfo) *Context {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)

	c := &Context{
		appInfo: &configv1.AppInfo{},
	}

	c.copyAppInfo(ai)
	c.InitAppInfo("", "", "", "", "")

	// 其余初始化例如 RootCtx/Cancel/Logger 可在这里设置
	_ = cancel // 保留 cancel 给调用者或另行设置
	_ = ctx
	return c
}

// NewContextWithParam 使用指定配置和 logger 创建引导上下文。
func NewContextWithParam(parent context.Context, ai *configv1.AppInfo, cfg *configv1.Bootstrap, log *slog.Logger) *Context {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)

	c := &Context{
		appInfo: &configv1.AppInfo{},
		config:  cfg,
		logger:  log,
	}

	c.copyAppInfo(ai)
	c.InitAppInfo("", "", "", "", "")

	// 其余初始化例如 RootCtx/Cancel/Logger 可在这里设置
	_ = cancel // 保留 cancel 给调用者或另行设置
	_ = ctx
	return c
}

// InitAppInfo 按启动参数、传入配置、默认值的优先级初始化应用信息。
func (c *Context) InitAppInfo(project, appID, instanceID, name, version string) {
	if c == nil {
		return
	}

	info := &configv1.AppInfo{}
	if c.appInfoConfig != nil {
		if clone, ok := proto.Clone(c.appInfoConfig).(*configv1.AppInfo); ok {
			info = clone
		}
	}

	if project != "" {
		info.Project = project
	}
	if appID != "" {
		info.AppId = appID
	}
	if instanceID != "" {
		info.InstanceId = instanceID
	}
	if name != "" {
		info.Name = name
	}
	if version != "" {
		info.Version = version
	}

	AdjustAppInfo(info)
	c.appInfo = info
}

// Context 返回应用级根 context（保证非 nil）
func (c *Context) Context() context.Context {
	if c == nil || c.rootCtx == nil {
		return context.Background()
	}
	return c.rootCtx
}

// CancelContext 触发取消（幂等）
func (c *Context) CancelContext() {
	if c == nil {
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
}

// NewLoggerHelper 创建带模块字段的 logger。
func (c *Context) NewLoggerHelper(moduleName string) *slog.Logger {
	if c == nil || c.logger == nil {
		return slog.Default().With("module", moduleName)
	}
	return c.logger.With("module", moduleName)
}

// GetLogger 返回当前引导上下文中的 logger。
func (c *Context) GetLogger() *slog.Logger {
	return c.logger
}

// GetConfig 返回当前的 *configv1.bootstrap（并发安全）
func (c *Context) GetConfig() *configv1.Bootstrap {
	if c.config == nil {
		return nil
	}
	if clone := proto.Clone(c.config); clone != nil {
		if b, ok := clone.(*configv1.Bootstrap); ok {
			return b
		}
	}
	return nil
}

func (c *Context) GetAppInfo() *configv1.AppInfo {
	if c.appInfo == nil {
		return nil
	}
	if clone := proto.Clone(c.appInfo); clone != nil {
		if a, ok := clone.(*configv1.AppInfo); ok {
			return a
		}
	}
	return nil
}

// setAppInfo 用受控方式替换整个 appInfo（可选）
func (c *Context) setAppInfo(src *configv1.AppInfo) {
	if c == nil || src == nil {
		return
	}
	if clone, ok := proto.Clone(src).(*configv1.AppInfo); ok {
		c.appInfoConfig = clone
	}
	c.InitAppInfo("", "", "", "", "")
}

// copyAppInfo 复制应用信息
func (c *Context) copyAppInfo(ai *configv1.AppInfo) {
	if ai == nil {
		return
	}

	// 保留调用方原始值，避免默认值提前写入后阻断启动参数覆盖。
	if clone, ok := proto.Clone(ai).(*configv1.AppInfo); ok {
		c.appInfoConfig = clone
	}
}

func (c *Context) PrintAppInfo() {
	ai := c.GetAppInfo()
	if ai == nil {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	host, _ := os.Hostname()
	pid := os.Getpid()

	if os.Getenv("APPINFO_FORMAT") == "json" {
		out := map[string]interface{}{
			"timestamp":   ts,
			"host":        host,
			"pid":         pid,
			"name":        ai.Name,
			"version":     ai.Version,
			"app_id":      ai.AppId,
			"instance_id": ai.InstanceId,
			"metadata":    ai.Metadata,
		}
		if b, err := json.Marshal(out); err == nil {
			fmt.Println(string(b))
		} else {
			fmt.Printf("Application info marshal error: %v\n", err)
		}
		return
	}

	fmt.Printf("[%s] %s (pid:%d@%s)\n", ts, ai.Name, pid, host)
	fmt.Printf("  Version: %s\n", ai.Version)
	fmt.Printf("  AppId: %s\n", ai.AppId)
	fmt.Printf("  InstanceId: %s\n", ai.InstanceId)
	if len(ai.Metadata) > 0 {
		fmt.Println("  Metadata:")
		keys := make([]string, 0, len(ai.Metadata))
		for k := range ai.Metadata {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			fmt.Printf("    %s=%s\n", k, ai.Metadata[k])
		}
	}
}

func (c *Context) GetRegistrar() kratosRegistry.Registrar {
	return c.registrar
}

// RegisterCustomConfig 注册自定义配置
func (c *Context) RegisterCustomConfig(key string, cfg proto.Message) {
	if key == "" || cfg == nil {
		return
	}

	if _, ok := c.customConfig.Load(key); ok {
		return
	}

	c.customConfig.Store(key, cfg)

	bConfig.RegisterConfig(cfg)
}

// SetCustomConfig 存入自定义配置
func (c *Context) SetCustomConfig(key string, cfg proto.Message) {
	if key == "" || cfg == nil {
		return
	}

	c.customConfig.Store(key, cfg)
}

// GetCustomConfig 获取自定义配置（原始类型）
func (c *Context) GetCustomConfig(key string) (any, bool) {
	return c.customConfig.Load(key)
}

// DeleteCustomConfig 删除自定义配置
func (c *Context) DeleteCustomConfig(key string) {
	c.customConfig.Delete(key)
}

// RangeCustomConfig 遍历自定义配置，回调返回 false 可停止遍历
func (c *Context) RangeCustomConfig(fn func(key string, val any) bool) {
	c.customConfig.Range(func(k, v any) bool {
		ks, _ := k.(string)
		return fn(ks, v)
	})
}

// SetValue 将任意值放入通用存储
func (c *Context) SetValue(key string, val interface{}) {
	c.values.Store(key, val)
}

// GetValue 从通用存储读取值
func (c *Context) GetValue(key string) (interface{}, bool) {
	return c.values.Load(key)
}

// UpTime 返回应用已运行时间
func (c *Context) UpTime() time.Duration {
	if c == nil || c.appInfo == nil {
		return 0
	}
	start := time.Unix(c.appInfo.StartTime.AsTime().Unix(), 0)
	return time.Since(start)
}
