package redis

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/utils"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/bsm/redislock"
)

type Redis struct {
	client *redis.Client
	mutex  *redislock.Client
}

// NewRedis 创建 Redis 分布式锁实例。
func NewRedis(cfg *conf.Data_Redis) (*Redis, func(), error) {
	redisOptions, err := utils.GetRedisOptions(cfg)
	if err != nil {
		log.Fatal("redis options failed", err)
		panic(err)
	}
	client := redis.NewClient(redisOptions)
	if client == nil {
		log.Fatalf("failed opening connection to redis")
	}

	// open tracing instrumentation.
	if cfg.GetEnableTracing() {
		if err = redisotel.InstrumentTracing(client); err != nil {
			log.Fatalf("failed open tracing: %s", err.Error())
		}
	}

	// open metrics instrumentation.
	if cfg.GetEnableMetrics() {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			log.Fatalf("failed open metrics: %s", err.Error())
		}
	}

	// 连接
	if _, err = client.Ping(context.TODO()).Result(); err != nil {
		log.Fatalf("failed ping redis: %s", err.Error())
	}
	return &Redis{
			client: client,
		}, func() {
			if client != nil {
				err = client.Close()
				if err != nil {
					log.Errorf("failed close redis: %s", err.Error())
					return
				}
			}
		}, nil
}

func (r *Redis) Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error) {
	if r.mutex == nil {
		r.mutex = redislock.New(r.client)
	}
	return r.mutex.Obtain(context.TODO(), key, time.Duration(ttl)*time.Second, options)
}
