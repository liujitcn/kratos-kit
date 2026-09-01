package utils

import (
	"errors"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/redis/go-redis/v9"
)

// GetRedisOptions 构造单机 Redis 连接配置。
func GetRedisOptions(cfg *configv1.Data_Redis) (*redis.Options, error) {
	if cfg == nil {
		return nil, errors.New("Redis 配置不能为空")
	}
	if len(cfg.GetAddr()) == 0 || cfg.GetAddr()[0] == "" {
		return nil, errors.New("Redis 地址不能为空")
	}
	redisOptions := &redis.Options{
		Addr:         cfg.GetAddr()[0],
		Password:     cfg.GetPassword(),
		DB:           int(cfg.GetDb()),
		DialTimeout:  cfg.GetDialTimeout().AsDuration(),
		ReadTimeout:  cfg.GetReadTimeout().AsDuration(),
		WriteTimeout: cfg.GetWriteTimeout().AsDuration(),
	}
	var err error
	redisOptions.TLSConfig, err = LoadServerTlsConfig(cfg.GetTls())
	return redisOptions, err
}

// GetClusterRedisOptions 构造 Redis 集群连接配置。
func GetClusterRedisOptions(cfg *configv1.Data_Redis) (*redis.ClusterOptions, error) {
	redisOptions := &redis.ClusterOptions{
		Addrs:        cfg.Addr,
		Password:     cfg.Password,
		DialTimeout:  cfg.DialTimeout.AsDuration(),
		ReadTimeout:  cfg.ReadTimeout.AsDuration(),
		WriteTimeout: cfg.WriteTimeout.AsDuration(),
	}
	var err error
	redisOptions.TLSConfig, err = LoadServerTlsConfig(cfg.Tls)
	return redisOptions, err
}

// GetUniversalOptions 构造通用 Redis 连接配置。
func GetUniversalOptions(cfg *configv1.Data_Redis) (*redis.UniversalOptions, error) {
	redisOptions := &redis.UniversalOptions{
		Addrs:        cfg.Addr,
		Password:     cfg.Password,
		DB:           int(cfg.Db),
		DialTimeout:  cfg.DialTimeout.AsDuration(),
		ReadTimeout:  cfg.ReadTimeout.AsDuration(),
		WriteTimeout: cfg.WriteTimeout.AsDuration(),
	}
	var err error
	redisOptions.TLSConfig, err = LoadServerTlsConfig(cfg.Tls)
	return redisOptions, err
}
