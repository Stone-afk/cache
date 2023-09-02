package cache

import (
	"context"
	"time"
)

type Option func(cache Cache)

// 值的问题
// - string: 可以，问题是本地缓存，结构体转化为 string，比如用 json 表达 User
// - []byte: 最通用的表达，可以存储序列化后的数据，也可以存储加密数据，还可以存储压缩数据。用户用起来不方便
// - any: Redis 之类的实现，你要考虑序列化的问题

type Cache interface {
	// Get a cached value by key.
	Get(ctx context.Context, key string) (any, error)
	// GetMulti is a batch version of Get.
	GetMulti(ctx context.Context, keys []string) ([]any, error)
	// Set a cached value by key.
	// Set(ctx context.Context, key string, val []byte, expiration time.Duration) error
	// millis 毫秒数，过期时间
	// Set(key string, val any, mills int64)
	Set(ctx context.Context, key string, val any, expiration time.Duration) error
	// Delete cached value by key.
	// Should not return error if key not found
	// 同时会把被删除的数据返回
	// Delete(key string) (any, error)
	Delete(ctx context.Context, key string) error
	// LoadAndDelete get and delete cached value by key
	LoadAndDelete(ctx context.Context, key string) (any, error)

	// Incr Increment a cached int value by key, as a counter.
	Incr(ctx context.Context, key string) error
	// Decr Decrement a cached int value by key, as a counter.
	Decr(ctx context.Context, key string) error
	// IsExist Check if a cached value exists or not.
	// if key is expired, return (false, nil)
	IsExist(ctx context.Context, key string) (bool, error)
	// ClearAll Clear all cache.
	ClearAll(ctx context.Context) error
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
