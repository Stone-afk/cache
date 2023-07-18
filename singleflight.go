package cache

import (
	"cache/internal/errs"
	"context"
	"golang.org/x/sync/singleflight"
	"log"
	"time"
)

var cache Cache

var group = &singleflight.Group{}

func QueryFromDB(key string) (any, error) {
	panic("implement")
}

// Biz -> singleflight + cache aside
// 普通的 singleflight 是和 cache aside 一起使用的。
// 在下面代码块中，业务代码发现缓存返回了
// errKeyNotFound，于是开始利用 singleflight 设计模式
// 去数据库加载数据，并且刷新缓存。
func Biz(key string) (any, error) {
	val, err := cache.Get(context.Background(), key)
	if err != nil && err != errs.ErrKeyNotFound {
		return nil, err
	}
	if err == errs.ErrKeyNotFound {
		val, err, _ := group.Do(key, func() (interface{}, error) {
			newVal, err := QueryFromDB(key)
			if err != nil {
				return nil, err
			}
			// _ = cache.Set(context.Background(), key, newVal, time.Minute)
			err = cache.Set(context.Background(), key, newVal, time.Minute)
			if err != nil {
				log.Fatalln(err)
			}
			return newVal, nil
		})
		return val, err
	}
	return val, nil
}

// singleflight + read through
// singleflight 也可以和 read through 结合，
// 做成一个装饰器模式。
// 本身 read through 也是一个装饰器模式。

// SingleflightCacheV1 也是装饰器模式
// 进一步封装 ReadThroughCache
// 在加载数据并且刷新缓存的时候应用了 singleflight 模式
type SingleflightCacheV1 struct {
	ReadThroughCache
}

// NewSingleflightCacheV1
// 这种实现是通过 singleflight 封装了
// LoadFunc 方法，可以确保从数据库加载数
// 据必然每个 key 一个 goroutine。但是把数据回写缓存就是多个 goroutine 重
// 复执行了。
func NewSingleflightCacheV1(cache Cache,
	LoadFunc func(ctx context.Context, key string) (any, error)) *SingleflightCacheV1 {
	g := &singleflight.Group{}
	return &SingleflightCacheV1{
		ReadThroughCache: ReadThroughCache{
			Cache:      cache,
			Expiration: time.Minute,

			LoadFunc: func(ctx context.Context, key string) (any, error) {
				defer func() {
					g.Forget(key)
				}()
				// 多个 goroutine 进来这里, 只有一个 goroutine 会真的去执行

				val, err, _ := g.Do(key, func() (interface{}, error) {
					return LoadFunc(ctx, key)
				})
				return val, err
			},
		},
	}
}

type SingleflightCacheV2 struct {
	ReadThroughCache
	group *singleflight.Group
}

// NewSingleflightCacheV2
// 这种就是很简单的装饰器模式，在 Get 方法
// 里面利用 singleflight 来完成加载数据 和 回写缓存两个步骤。
func NewSingleflightCacheV2(cache Cache,
	LoadFunc func(ctx context.Context, key string) (any, error)) *SingleflightCacheV2 {
	return &SingleflightCacheV2{
		ReadThroughCache: ReadThroughCache{
			Cache:    cache,
			LoadFunc: LoadFunc,
		},
		group: &singleflight.Group{},
	}
}

func (s *SingleflightCacheV2) Get(ctx context.Context, key string) (any, error) {
	val, err := s.Cache.Get(ctx, key)
	if err != nil && err != errs.ErrKeyNotFound {
		return nil, err
	}
	if err == errs.ErrKeyNotFound {
		defer func() {
			s.group.Forget(key)
		}()
		val, err, _ = s.group.Do(key, func() (interface{}, error) {

			v, exc := s.LoadFunc(ctx, key)
			if exc == nil {
				if e := s.Cache.Set(ctx, key, v, s.Expiration); e != nil {
					log.Fatalf("刷新缓存失败, err: %v", err)
				}
			}
			return v, exc
		})
	}
	return val, err
}
