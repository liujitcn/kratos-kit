package queue

import (
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/queue/memory"
	"github.com/liujitcn/kratos-kit/queue/redis"
)

type Queue interface {
	Append(stream string, message data.Message) error
	Register(stream string, fn data.ConsumerFunc)
	Run()
	Shutdown()
}

func NewQueue(redisConf *conf.Data_Redis, queueConf *conf.Data_Queue) (Queue, func(), error) {
	var queue Queue
	if redisConf == nil {
		var poolSize int64
		if queueConf != nil && queueConf.Memory != nil {
			poolSize = queueConf.Memory.PoolSize
		}
		queue = memory.NewMemory(poolSize)
	} else {
		queue = redis.NewRedis(redisConf, queueConf)
	}
	if queue == nil {
		return nil, nil, errors.New("queue is null")
	}
	return queue, func() {
		log.Info("queue cleanup...")
		queue.Shutdown()
	}, nil
}
