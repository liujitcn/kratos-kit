package cache

import (
	"errors"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// ReadRevision 读取共享缓存版本；版本键不存在时返回初始版本0。
func ReadRevision(store Cache, key string) (string, bool) {
	if store == nil {
		return "", false
	}
	revision, err := store.Get(key)
	if err == nil {
		if _, err = strconv.ParseInt(revision, 10, 64); err != nil {
			return "", false
		}
		return revision, true
	}
	message := strings.ToLower(err.Error())
	if errors.Is(err, redis.Nil) || strings.Contains(message, "not found") || strings.Contains(message, "key expired") {
		return "0", true
	}
	return "", false
}

// IncrementRevision 原子递增共享缓存版本，使所有节点切换到新快照。
func IncrementRevision(store Cache, key string) error {
	if store == nil {
		return nil
	}
	_, err := store.Incr(key)
	return err
}
