package redisqueue

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var redisVersionRE = regexp.MustCompile(`redis_version:(.+)`)

// RedisOptions 是 redis.UniversalOptions 的别名，便于调用方直接复用配置结构。
type RedisOptions = redis.UniversalOptions

// newRedisClient 根据给定配置创建 Redis 客户端；若配置为空，则使用默认配置。
func newRedisClient(options *RedisOptions) redis.UniversalClient {
	if options == nil {
		options = &RedisOptions{}
	}
	return redis.NewUniversalClient(options)
}

// newCheckedRedisClient 创建并校验 Redis 客户端，若校验失败则立即释放底层连接资源。
func newCheckedRedisClient(options *RedisOptions) (redis.UniversalClient, error) {
	client := newRedisClient(options)
	if err := redisPreflightChecks(client); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

// redisPreflightChecks 校验 Redis 实例是否可连接，并确认当前版本支持 Redis Streams。
func redisPreflightChecks(client redis.UniversalClient) error {
	info, err := client.Info(context.TODO(), "server").Result()
	if err != nil {
		return err
	}

	match := redisVersionRE.FindAllStringSubmatch(info, -1)
	if len(match) < 1 {
		return fmt.Errorf("could not extract redis version")
	}
	version := strings.TrimSpace(match[0][1])
	parts := strings.Split(version, ".")
	var major int
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	if major < 5 {
		return fmt.Errorf("redis streams are not supported in version %q", version)
	}

	return nil
}

// incrementMessageID 将消息 ID 的序号部分加一，用于继续向后分页读取消息。
func incrementMessageID(id string) (string, error) {
	parts := strings.Split(id, "-")
	index := parts[1]
	parsed, err := strconv.ParseInt(index, 10, 64)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing message ID %q", id)
	}
	return fmt.Sprintf("%s-%d", parts[0], parsed+1), nil
}
