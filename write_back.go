package cache

import (
	"context"
	"log"
	"time"
)

type WriteBackCache struct {
	*LocalCache
}

func NewWriteBackCache(StoreFunc func(ctx context.Context, key string, val any) error) (*WriteBackCache, error) {
	cache, err := NewLocalCache(
		LocalCacheWithOnEvicteds(func(ctx context.Context, key string, val any) error {
			// 这个地方，context 不好设置
			// error 不好处理
			err := StoreFunc(ctx, key, val)
			if err != nil {
				log.Fatalln(err)
			}
			return nil
			// return Store(ctx, key, val)
		}))

	if err != nil {
		return nil, err
	}
	return &WriteBackCache{
		LocalCache: cache,
	}, nil
}

func (w *WriteBackCache) Close() error {
	// 遍历所有的 key，将值刷新到数据库
	return w.LocalCache.Close(context.Background())
}

// PreloadCache 预加载
type PreloadCache struct {
	Cache
	sentinelCache *LocalCache
}

func NewPreloadCache(c Cache, LoadFunc func(ctx context.Context, key string) (any, error)) (*PreloadCache, error) {
	localCache, err := NewLocalCache(
		LocalCacheWithOnEvicteds(func(ctx context.Context, key string, val any) error {
			val, err := LoadFunc(ctx, key)
			if err != nil {
				return err
			}
			err = c.Set(ctx, key, val, time.Minute)
			if err != nil {
				log.Fatalln(err)
			}
			return nil
			// return Store(ctx, key, val)
		}))
	if err != nil {
		return nil, err
	}
	return &PreloadCache{
		Cache:         c,
		sentinelCache: localCache,
	}, nil
}

func (c *PreloadCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	// sentinelExpiration 的设置原则是：
	// 确保 expiration - sentinelExpiration 这段时间内，来得及加载数据刷新缓存
	// 要注意 OnEvicted 的时机，尤其是懒删除，但是轮询删除效果又不是很好的时候
	sentinelExpiration := expiration - time.Second*5
	err := c.sentinelCache.Set(ctx, key, "", sentinelExpiration)
	if err != nil {
		log.Fatalln(err)
	}
	return c.Cache.Set(ctx, key, val, expiration)
}
