package cache

import (
	"context"
	"fmt"
	"time"
)

type WriteThroughCache struct {
	Cache
	StoreFunc func(ctx context.Context, key string, val any) error
}

func (c *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	err := c.StoreFunc(ctx, key, val)
	if err != nil {
		return fmt.Errorf("cache 无法存储数据: %w", err)
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
