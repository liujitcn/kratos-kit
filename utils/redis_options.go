package utils

import (
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/redis/go-redis/v9"
)

func GetRedisOptions(cfg *conf.Data_Redis) (*redis.Options, error) {
	redisOptions := &redis.Options{
		Addr:         cfg.Addr[0],
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

func GetClusterRedisOptions(cfg *conf.Data_Redis) (*redis.ClusterOptions, error) {
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

func GetUniversalOptions(cfg *conf.Data_Redis) (*redis.UniversalOptions, error) {
	redisOptions := &redis.UniversalOptions{
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
