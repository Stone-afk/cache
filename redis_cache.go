package cache

import (
	"cache/internal/errs"
	"context"
	"fmt"
	"github.com/go-redis/redis/v9"
	"time"
)

var DefaultKey = "cacheRedis"

type RedisCache struct {
	key string
	cmd redis.Cmdable
}

type RedisCacheOption func(r *RedisCache)

func WithKeyOption(key string) RedisCacheOption {
	return func(r *RedisCache) {
		r.key = key
	}
}

func NewRedisCache(client redis.Cmdable, opts ...RedisCacheOption) *RedisCache {
	res := &RedisCache{
		key: DefaultKey,
		cmd: client,
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
	return r.cmd.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	msg, err := r.cmd.Set(ctx, key, val, expiration).Result()
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
	_, err := r.cmd.Del(ctx, key).Result()
	return err
}

func (r *RedisCache) LoadAndDelete(ctx context.Context, key string) (any, error) {
	return r.cmd.GetDel(ctx, key).Result()
}

func (r *RedisCache) IsExist(ctx context.Context, key string) (bool, error) {
	res, err := r.cmd.Exists(ctx, key).Result()
	return res > 0, err
}

func (r *RedisCache) ClearAll(ctx context.Context) error {
	keys, err := r.Scan(ctx, r.key+":*")
	if err != nil {
		return err
	}
	_, err = r.cmd.Del(ctx, keys...).Result()
	return err
}

func (r *RedisCache) Scan(ctx context.Context, pattern string) (keys []string, err error) {
	var cursor uint64 = 0 // start
	for {
		ks, csor, err := r.cmd.Scan(ctx, cursor, pattern, 1024).Result()
		if err != nil {
			return
		}
		keys = append(keys, ks...)
		cursor = csor
	}
}

func (r *RedisCache) associate(originKey interface{}) string {
	return fmt.Sprintf("%s:%s", r.key, originKey)
}
