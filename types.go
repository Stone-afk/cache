package cache

import (
	"context"
	"errors"
	"time"
)

var (
	errKeyNotFound      = errors.New("cache: 找不到 key")
	errKeyExpired       = errors.New("cache: key 已经过期")
	errOverCapacity     = errors.New("cache: 超过缓存最大容量")
	errFailedToSetCache = errors.New("cache: 设置键值对失败")
	errInvalidkey       = errors.New("invalid key")
)

type Option func(cache Cache)

// 值的问题
// - string: 可以，问题是本地缓存，结构体转化为 string，比如用 json 表达 User
// - []byte: 最通用的表达，可以存储序列化后的数据，也可以存储加密数据，还可以存储压缩数据。用户用起来不方便
// - any: Redis 之类的实现，你要考虑序列化的问题

type Cache interface {
	// Get val, err  := Get(ctx)
	// str = val.(string)
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, val any, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	LoadAndDelete(ctx context.Context, key string) (any, error)

	// 作业在这里
	// OnEvicted(ctx context.Context) <- chan KV
}

type CacheV2[T any] interface {
	Get(ctx context.Context, key string) (T, error)

	Set(ctx context.Context, key string, val T, expiration time.Duration) error

	Delete(ctx context.Context, key string) error
}

// type CacheV3 interface {
// 	Get[T any](ctx context.Context, key string) (T, error)
//
// 	Set[T any](ctx context.Context, key string, val T, expiration time.Duration) error
//
// 	Delete(ctx context.Context, key string) error
// }
