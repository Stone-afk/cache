package cache

import (
	"cache/internal/errs"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type MaxCntLocalCache struct {
	*LocalCache
	MaxCnt int32
	Cnt    int32
	mutex  sync.Mutex
}

func NewMaxCntCache(c *LocalCache, maxCnt int32) *MaxCntLocalCache {
	res := &MaxCntLocalCache{
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

func (m *MaxCntLocalCache) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	// 加锁是为了防止多个 goroutine 同时 Get同一个 key 返回 errKeyNotFound，导致原子操作加两次1
	m.mutex.Lock()
	defer m.mutex.Unlock()
	_, err := m.LocalCache.Get(ctx, key)
	if err != nil && err != errs.ErrKeyNotFound {
		return err
	}
	// 避免重复的key算两次
	if err == errs.ErrKeyNotFound {
		// 判断有没有超过最大值
		cnt := atomic.AddInt32(&m.Cnt, 1)
		// 满了
		if cnt > m.MaxCnt {
			atomic.AddInt32(&m.Cnt, -1)
			return errs.ErrOverCapacity
		}
	}
	return m.LocalCache.Set(ctx, key, val, expiration)
}
