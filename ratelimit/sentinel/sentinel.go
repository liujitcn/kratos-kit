// Package sentinel 提供适用于 Kratos 服务端中间件的 Sentinel 限流器。
package sentinel

import (
	"errors"
	"fmt"

	sentinelapi "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	kratosRateLimit "github.com/go-kratos/kratos/v3/middleware/ratelimit"
)

// ErrEmptyResource 表示 Sentinel 资源名称为空。
var ErrEmptyResource = errors.New("ratelimit/sentinel: resource is empty")

var _ kratosRateLimit.Limiter = (*Limiter)(nil)

// Option 配置 Sentinel 限流器。
type Option func(*options)

type options struct {
	trafficType base.TrafficType
	entryOpts   []sentinelapi.EntryOption
}

type entryFunc func(string, ...sentinelapi.EntryOption) (*base.SentinelEntry, *base.BlockError)

// WithTrafficType 设置 Sentinel 流量类型。
func WithTrafficType(trafficType base.TrafficType) Option {
	return func(o *options) {
		o.trafficType = trafficType
	}
}

// WithEntryOptions 设置额外的 Sentinel Entry 选项。
func WithEntryOptions(entryOpts ...sentinelapi.EntryOption) Option {
	return func(o *options) {
		o.entryOpts = append([]sentinelapi.EntryOption(nil), entryOpts...)
	}
}

// Limiter 将 Sentinel 资源适配为 Kratos 限流器。
type Limiter struct {
	resource string
	options  *options
	entry    entryFunc
}

// New 创建 Sentinel 限流器。
//
// 调用方需要在启动期初始化 Sentinel，并为同名 resource 加载 flow 规则。
func New(resource string, opts ...Option) (*Limiter, error) {
	if resource == "" {
		return nil, ErrEmptyResource
	}
	cfg := &options{trafficType: base.Inbound}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Limiter{
		resource: resource,
		options:  cfg,
		entry:    sentinelapi.Entry,
	}, nil
}

// Allow 尝试进入 Sentinel 资源，并用 Kratos 完成回调结束本次 Entry。
func (l *Limiter) Allow() (kratosRateLimit.DoneFunc, error) {
	entryOpts := make([]sentinelapi.EntryOption, 0, 1+len(l.options.entryOpts))
	entryOpts = append(entryOpts, sentinelapi.WithTrafficType(l.options.trafficType))
	entryOpts = append(entryOpts, l.options.entryOpts...)
	entry, blockErr := l.entry(l.resource, entryOpts...)
	if blockErr != nil {
		return nil, fmt.Errorf("%w: %s", kratosRateLimit.ErrLimitExceed, blockErr.Error())
	}
	return func(info kratosRateLimit.DoneInfo) {
		if info.Err != nil {
			sentinelapi.TraceError(entry, info.Err)
		}
		entry.Exit()
	}, nil
}
