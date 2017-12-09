package models

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/soopsio/mgmt/config"
)

func NewRedisClient() *redis.Client {
	cfg := config.GetConfig()
	return redis.NewClient(&redis.Options{
		Addr:            cfg.DefaultString("redis::addr", "localhost:6379"),
		Password:        cfg.DefaultString("redis::password", ""), // no password set
		DB:              cfg.DefaultInt("redis::db", 0),           // use default DB
		PoolSize:        cfg.DefaultInt("redis::pool_size", 100),
		MaxRetries:      cfg.DefaultInt("redis::max_retries", 5),
		MinRetryBackoff: time.Duration(cfg.DefaultInt("redis::min_retry_backoff", 500)) * time.Millisecond,
		MaxRetryBackoff: time.Duration(cfg.DefaultInt("redis::max_retry_backoff", 5000)) * time.Millisecond,
		DialTimeout:     time.Duration(cfg.DefaultInt("redis::dial_timeout", 3000)) * time.Millisecond,
		ReadTimeout:     time.Duration(cfg.DefaultInt("redis::read_timeout", 3000)) * time.Millisecond,
		WriteTimeout:    time.Duration(cfg.DefaultInt("redis::write_timeout", 5000)) * time.Millisecond,
	})
}
