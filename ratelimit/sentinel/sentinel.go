package sentinel

import (
	"context"
	"errors"
	"fmt"
	"slices"

	sentinelapi "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
)

// ErrEmptyResource 表示 Sentinel 资源名称为空。
var ErrEmptyResource = errors.New("ratelimit/sentinel: resource is empty")

// ErrLimited 表示 Sentinel 拒绝了本次请求。
var ErrLimited = errors.New("rate limit exceeded")

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
		o.entryOpts = slices.Clone(entryOpts)
	}
}

// Limiter 将 Sentinel 资源适配为传输层限流器。
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

// Allow 尝试进入 Sentinel 资源并立即结束本次 Entry。
func (l *Limiter) Allow() (bool, error) {
	entryOpts := make([]sentinelapi.EntryOption, 0, 1+len(l.options.entryOpts))
	entryOpts = append(entryOpts, sentinelapi.WithTrafficType(l.options.trafficType))
	entryOpts = append(entryOpts, l.options.entryOpts...)
	entry, blockErr := l.entry(l.resource, entryOpts...)
	if blockErr != nil {
		return false, fmt.Errorf("%w: %s", ErrLimited, blockErr.Error())
	}
	entry.Exit()
	return true, nil
}

// Wait 检查 Sentinel 资源，Sentinel 本身不提供阻塞等待语义。
func (l *Limiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := l.Allow()
	return err
}
