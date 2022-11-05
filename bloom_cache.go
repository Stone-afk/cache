package cache

import (
	"context"
	"log"
	"time"
)

type BloomFilterCache struct {
	Cache
	BloomFilter
	LoadFunc func(ctx context.Context, key string) (any, error)
}

// BloomFilter 认为 key 存在，才会最终去数据库中查询。
// 大部分不存在的 key1 会直接在 BloomFilter 这一层被拦下。
type BloomFilter interface {
	HasKey(ctx context.Context, key string) (bool, error)
}

func NewBloomFilterCacheV(cache Cache, bloomFilter BloomFilter,
	loadFunc func(ctx context.Context, key string) (any, error)) *BloomFilterCache {
	return &BloomFilterCache{
		Cache:       cache,
		BloomFilter: bloomFilter,
		LoadFunc:    loadFunc}
}

func (s *BloomFilterCache) Get(ctx context.Context, key string) (any, error) {
	val, err := s.Cache.Get(ctx, key)
	if err != nil && err != errKeyNotFound {
		return nil, err
	}
	if err == errKeyNotFound {
		exist, _ := s.HasKey(ctx, key)
		if !exist {
			return nil, errInvalidkey
		}
		val, err = s.LoadFunc(ctx, key)
		if err == nil {
			if e := s.Set(ctx, key, val, time.Minute); e != nil {
				log.Fatalf("刷新缓存失败, err: %v", err)
			}
		}
	}
	return val, err
}

func NewBloomFilterCacheV1(cache Cache, bloomFilter BloomFilter,
	loadFunc func(ctx context.Context, key string) (any, error)) *BloomFilterCache {
	return &BloomFilterCache{
		Cache:       cache,
		BloomFilter: bloomFilter,
		LoadFunc: func(ctx context.Context, key string) (any, error) {
			ok, _ := bloomFilter.HasKey(ctx, key)
			if ok {
				return loadFunc(ctx, key)
			}
			return nil, errInvalidkey
		},
	}
}

func (s *BloomFilterCache) GetV1(ctx context.Context, key string) (any, error) {
	val, err := s.Cache.Get(ctx, key)
	if err != nil && err != errKeyNotFound {
		return nil, err
	}
	if err == errKeyNotFound {
		val, err = s.LoadFunc(ctx, key)
		if err == nil {
			if e := s.Set(ctx, key, val, time.Minute); e != nil {
				log.Fatalf("刷新缓存失败, err: %v", err)
			}
		}
	}
	return val, err
}

type LimitFilter interface {
	// 查看有无触发限流，有的话直接返回
	isLimit(ctx context.Context) bool
}

// 加了限流的实现
type LimitCache struct {
}
