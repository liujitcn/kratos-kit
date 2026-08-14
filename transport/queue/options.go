package queue

import (
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	basequeue "github.com/liujitcn/kratos-kit/queue"
)

type backendType uint8

const (
	backendMemory backendType = iota
	backendRedis
)

type options struct {
	backend    backendType
	memorySize int64
	redisConf  *configv1.Data_Redis
	queueConf  *configv1.Data_Queue
	instance   basequeue.Queue
}

// ServerOption 配置队列 transport。
type ServerOption func(*options)

// WithMemory 选择本地内存队列，并设置内存队列池大小。
func WithMemory(poolSize int64) ServerOption {
	return func(o *options) {
		o.backend = backendMemory
		o.memorySize = poolSize
	}
}

// WithRedis 选择 Redis 队列，并设置 Redis 与队列配置。
func WithRedis(redisConf *configv1.Data_Redis, queueConf *configv1.Data_Queue) ServerOption {
	return func(o *options) {
		o.backend = backendRedis
		o.redisConf = redisConf
		o.queueConf = queueConf
	}
}

// WithQueue 注入已有队列实例，便于复用队列或进行测试。
func WithQueue(instance basequeue.Queue) ServerOption {
	return func(o *options) {
		o.instance = instance
	}
}
