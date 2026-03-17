package queue

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/queue/memory"
	"github.com/liujitcn/kratos-kit/queue/redis"
)

// Queue 定义统一的队列操作接口。
type Queue interface {
	Append(stream string, message data.Message) error
	Register(stream string, fn data.ConsumerFunc)
	Run()
	Shutdown()
}

// NewQueue 根据配置创建内存或 Redis 队列实例。
func NewQueue(redisConf *conf.Data_Redis, queueConf *conf.Data_Queue) (Queue, func(), error) {
	var queue Queue
	var err error
	if redisConf == nil {
		var poolSize int64
		if queueConf != nil && queueConf.Memory != nil {
			poolSize = queueConf.Memory.PoolSize
		}
		queue = memory.NewMemory(poolSize)
	} else {
		queue, err = redis.NewRedis(redisConf, queueConf)
		if err != nil {
			return nil, nil, fmt.Errorf("init redis queue failed: %w", err)
		}
	}
	if queue == nil {
		return nil, nil, fmt.Errorf("queue is nil")
	}
	return queue, func() {
		log.Info("queue cleanup...")
		queue.Shutdown()
	}, nil
}
