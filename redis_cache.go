package cache

import (
	"cache/internal/errs"
	"context"
	"fmt"
	"github.com/go-redis/redis/v9"
	"time"
)

var DefaultKey = "cacheRedis"

var _ Cache = &RedisCache{}

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

//func NewRedisCacheV2(addr string) *RedisCache {
//	client := redis.NewClient(&redis.Options{
//		Addr:     addr,
//		Password: "", // no password set
//		DB:       0,  // use default DB
//	})
//	return NewRedisCache(client)
//}

func (rc *RedisCache) Get(ctx context.Context, key string) (any, error) {
	return rc.cmd.Get(ctx, key).Result()
}

func (rc *RedisCache) GetMulti(ctx context.Context, keys []string) ([]any, error) {
	return rc.cmd.MGet(ctx, keys...).Result()
}

func (rc *RedisCache) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	msg, err := rc.cmd.Set(ctx, key, val, expiration).Result()
	if err != nil {
		return err
	}
	if msg != "OK" {
		// 理论上来说这里永远不会进来
		return fmt.Errorf("%w, 返回信息 %s", errs.ErrFailedToSetCache, msg)
	}
	return nil
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	// 我们并不关心究竟有没有删除到东西
	_, err := rc.cmd.Del(ctx, key).Result()
	return err
}

func (rc *RedisCache) LoadAndDelete(ctx context.Context, key string) (any, error) {
	return rc.cmd.GetDel(ctx, key).Result()
}

func (rc *RedisCache) Incr(ctx context.Context, key string) error {
	return rc.cmd.Incr(ctx, key).Err()
}

func (rc *RedisCache) Decr(ctx context.Context, key string) error {
	return rc.cmd.Decr(ctx, key).Err()
}

func (rc *RedisCache) IsExist(ctx context.Context, key string) (bool, error) {
	res, err := rc.cmd.Exists(ctx, key).Result()
	return res > 0, err
}

func (rc *RedisCache) ClearAll(ctx context.Context) error {
	keys, err := rc.Scan(ctx, rc.key+":*")
	if err != nil {
		return err
	}
	_, err = rc.cmd.Del(ctx, keys...).Result()
	return err
}

func (rc *RedisCache) Scan(ctx context.Context, pattern string) (keys []string, err error) {
	var cursor uint64 = 0 // start
	for {
		ks, csor, err := rc.cmd.Scan(ctx, cursor, pattern, 1024).Result()
		if err != nil {
			return
		}
		keys = append(keys, ks...)
		cursor = csor
	}
}

func (rc *RedisCache) associate(originKey interface{}) string {
	return fmt.Sprintf("%s:%s", rc.key, originKey)
}
