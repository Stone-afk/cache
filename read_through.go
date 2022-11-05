package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// type AsyncReadThroughCache struct {
// 	ReadThroughCache
// }

// ReadThroughCache 是一个装饰器
// 在原本 Cache 的功能上添加了 read through 功能
type ReadThroughCache struct {
	Cache
	mutex      sync.RWMutex
	Expiration time.Duration
	LoadFunc   func(ctx context.Context, key string) (any, error)
}

func (c *ReadThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.RUnlock()
	return c.Cache.Set(ctx, key, val, expiration)
}

func (c *ReadThroughCache) Get(ctx context.Context, key string) (any, error) {
	c.mutex.RLock()
	val, err := c.Cache.Get(ctx, key)
	c.mutex.RUnlock()
	if err != nil && err != errKeyNotFound {
		return nil, err
	}
	if err == errKeyNotFound {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		// 加锁问题
		// 两个 goroutine 进来这里
		val, err = c.LoadFunc(ctx, key)

		// 第一个 key1=value1
		// 中间有人更新了数据库
		// 第二个 key1=value2

		if err != nil {
			return nil, fmt.Errorf("cache 无法加载数据: %w", err)
		}

		// 这里 err 可以考虑忽略掉，或者输出 warn 日志
		err = c.Cache.Set(ctx, key, val, c.Expiration)
		// 可能的结果: goroutine1 先，毫无问题，数据库和缓存是一致的
		// goroutine2 先，那就有问题了, DB 是 value2，但是 cache 是 value1
		if err != nil {
			log.Fatalln(err)
		}

		// 这里可以并不关心 缓存是否成功写入
		//_ = c.Cache.Set(ctx, key, val, c.Expiration)
	}
	return val, nil
}

// Get 全异步
//func (c *ReadThroughCache) Get(ctx context.Context, key string) (any, error) {
//	c.mutex.RLock()
//	val, err := c.Cache.Get(ctx, key)
//	c.mutex.RUnlock()
//	if err != nil && err != errKeyNotFound {
//		return nil, err
//	}
//	if err == errKeyNotFound {
//		go func() {
//			c.mutex.Lock()
//			defer c.mutex.Unlock()
//
//			val, err = c.LoadFunc(ctx, key)
//
//			if err != nil {
//				log.Fatalln(err)
//			}
//
//			err = c.Cache.Set(ctx, key, val, c.Expiration)
//			if err != nil {
//				log.Fatalln(err)
//			}
//
//		}()
//	}
//	return val, nil
//}

// Get 半异步
//func (c *ReadThroughCache) Get(ctx context.Context, key string) (any, error) {
//	c.mutex.RLock()
//	val, err := c.Cache.Get(ctx, key)
//	c.mutex.RUnlock()
//	if err != nil && err != errKeyNotFound {
//		return nil, err
//	}
//	if err == errKeyNotFound {
//		c.mutex.Lock()
//		defer c.mutex.Unlock()
//
//		val, err = c.LoadFunc(ctx, key)
//
//		if err != nil {
//			return nil, fmt.Errorf("cache 无法加载数据: %w", err)
//		}
//
//		go func() {
//			err = c.Cache.Set(ctx, key, val, c.Expiration)
//
//			if err != nil {
//				log.Fatalln(err)
//			}
//		}()
//
//		return val, nil
//	}
//	return val, nil
//}

var _ Cache = &ReadThroughCacheV1[any]{}

// ReadThroughCacheV1 使用泛型会直接报错
// var c Cache= &ReadThroughCacheV1[*User]{} 编译无法通过
type ReadThroughCacheV1[T any] struct {
	Cache
	mutex      sync.RWMutex
	Expiration time.Duration
	// 我们把最常见的”捞DB”这种说法抽象为”加载数据”
	LoadFunc func(ctx context.Context, key string) (T, error)
}

func (c *ReadThroughCacheV1[T]) Set(ctx context.Context, key string, val T, expiration time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.RUnlock()
	return c.Cache.Set(ctx, key, val, expiration)
}

func (c *ReadThroughCacheV1[T]) Get(ctx context.Context, key string) (T, error) {
	c.mutex.RLock()
	val, err := c.Cache.Get(ctx, key)
	c.mutex.RUnlock()
	if err != nil && err != errKeyNotFound {
		return nil, err
	}
	if err == errKeyNotFound {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		val, err = c.LoadFunc(ctx, key)

		if err != nil {
			return nil, fmt.Errorf("cache 无法加载数据: %w", err)
		}

		// 忽略缓存刷新失败
		err = c.Cache.Set(ctx, key, val, c.Expiration)
		if err != nil {
			log.Fatalln(err)
		}
	}

	var t T
	if val != nil {
		t = val.(T)
	}
	return t, nil
}

func (c *ReadThroughCacheV1[T]) Delete(ctx context.Context, key string) error {
	return c.Cache.Delete(ctx, key)
}

func (c *ReadThroughCacheV1[T]) LoadAndDelete(ctx context.Context, key string) (T, error) {
	val, err := c.Cache.LoadAndDelete(ctx, key)
	var t T
	if val != nil {
		t = val.(T)
	}
	return t, err

}

type ReadThroughCacheV2[T any] struct {
	CacheV2[T]
	mutex      sync.RWMutex
	Expiration time.Duration
	// 我们把最常见的”捞DB”这种说法抽象为”加载数据”
	LoadFunc func(ctx context.Context, key string) (T, error)
}

type ReadThroughCacheV3 struct {
	Cache
	mutex      sync.RWMutex
	Expiration time.Duration
	// 我们把最常见的”捞DB”这种说法抽象为”加载数据”
	// LoadFunc func(ctx context.Context, key string) (any, error)
	Loader
}

type Loader interface {
	Load(ctx context.Context, key string) (any, error)
}

type LoadFunc func(ctx context.Context, key string) (any, error)

func (l LoadFunc) Load(ctx context.Context, key string) (any, error) {
	return l(ctx, key)
}
