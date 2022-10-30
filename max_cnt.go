package cache

import (
	"context"
	"sync/atomic"
	"time"
)

type MaxCntCacheDecorator struct {
	*LocalCache
	MaxCnt int32
	Cnt    int32
}

func NewMaxCntCache(c *LocalCache, maxCnt int32) *MaxCntCacheDecorator {
	res := &MaxCntCacheDecorator{
		MaxCnt:     maxCnt,
		LocalCache: c,
	}
	onEvicted := func(ctx context.Context, key string, val any) error {
		atomic.AddInt32(&res.Cnt, -1)
		return nil
	}
	LocalCacheWithOnEvicteds(onEvicted)(c)
	return res
}

func (m *MaxCntCacheDecorator) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	cnt := atomic.AddInt32(&m.Cnt, 1)
	if cnt > m.MaxCnt {
		atomic.AddInt32(&m.Cnt, -1)
		return errOverCapacity
	}
	return m.LocalCache.Set(ctx, key, val, expiration)
}
