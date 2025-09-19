package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thiagozs/go-exchange/internal/logger"
)

type RedisCache struct {
	client *redis.Client
	log    *logger.Logger
}

func New(addr string, db int, username, password string, log *logger.Logger) *RedisCache {
	r := redis.NewClient(&redis.Options{
		Addr:     addr,
		DB:       db,
		Username: username,
		Password: password,
	})
	return &RedisCache{client: r, log: log}
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		r.log.WithContext(ctx).Debugf("cache miss: %s", key)
		return "", nil
	} else if err != nil {
		r.log.WithContext(ctx).Errorf("cache error: %v", err)
		return "", err
	}
	r.log.WithContext(ctx).Debugf("cache hit: %s", key)
	return val, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		r.log.WithContext(ctx).Errorf("cache set error: %v", err)
	} else {
		r.log.WithContext(ctx).Debugf("cache set: %s", key)
	}
	return err
}
