package cache

import (
	"cache/internal/errs"
	"context"
	"fmt"
	"github.com/go-redis/redis/v9"
	"time"
)

type RedisCache struct {
	client redis.Cmdable
}

type RedisCacheOption func(r *RedisCache)

func NewRedisCache(client redis.Cmdable, opts ...RedisCacheOption) *RedisCache {
	res := &RedisCache{
		client: client,
	}
	for _, opt := range opts {
		opt(res)
	}
	return res
}

func NewRedisCacheV2(addr string) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return NewRedisCache(client)
}

func (r *RedisCache) Get(ctx context.Context, key string) (any, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	msg, err := r.client.Set(ctx, key, val, expiration).Result()
	if err != nil {
		return err
	}
	if msg != "OK" {
		// 理论上来说这里永远不会进来
		return fmt.Errorf("%w, 返回信息 %s", errs.ErrFailedToSetCache, msg)
	}
	return nil
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	// 我们并不关心究竟有没有删除到东西
	_, err := r.client.Del(ctx, key).Result()
	return err
}

func (r *RedisCache) LoadAndDelete(ctx context.Context, key string) (any, error) {
	return r.client.GetDel(ctx, key).Result()
}
