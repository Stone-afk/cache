package cache

import (
	"cache/internal/errs"
	"context"
	"time"
)

type WriteThroughCache struct {
	Cache
	storeFunc func(ctx context.Context, key string, val any) error
}

// NewWriteThroughCache creates a write through cache pattern decorator.
// The fn is the function that persistent the key and val.
func NewWriteThroughCache(cache Cache, fn func(ctx context.Context, key string, val any) error) (*WriteThroughCache, error) {
	if fn == nil || cache == nil {
		return nil, errs.ErrStoreFuncRequired
	}

	return &WriteThroughCache{
		Cache:     cache,
		storeFunc: fn,
	}, nil
}

func (c *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	err := c.storeFunc(ctx, key, val)
	if err != nil {
		return errs.ErrCannotStore(err)
	}
	return c.Cache.Set(ctx, key, val, expiration)

	// err :=
	// if err != nil {
	// 	return err
	// }

	// 万一我这里失败了呢？我要不要把缓存删掉？
	// err =  w.StoreFunc(ctx, key, val)
	// if err != nil {
	// 	w.Delete(ctx, key)
	// }
}

// Set 全异步
//func (c *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
//	go func() {
//		err := c.StoreFunc(ctx, key, val)
//		if err != nil {
//			log.Fatalln(err)
//
//		}
//		err = c.Cache.Set(ctx, key, val, expiration)
//		if err != nil {
//			log.Fatalln(err)
//		}
//	}()
//	return nil
//}

// Set 半异步
//func (c *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
//
//	err := c.StoreFunc(ctx, key, val)
//	if err != nil {
//		return fmt.Errorf("cache 无法存储数据: %w", err)
//	}
//	go func() {
//		err = c.Cache.Set(ctx, key, val, expiration)
//		if err != nil {
//			log.Fatalln(err)
//		}
//	}()
//	return nil
//}
