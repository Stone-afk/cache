package cache

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"
)

type RandomExpireCache struct {
	Cache
	offset func() time.Duration
}

// RandomExpireCacheOption implement genreate random time offset expired option
type RandomExpireCacheOption func(*RandomExpireCache)

// WithRandomExpireOffsetFunc returns a RandomExpireCacheOption that configures the offset function
func WithRandomExpireOffsetFunc(fn func() time.Duration) RandomExpireCacheOption {
	return func(cache *RandomExpireCache) {
		cache.offset = fn
	}
}

func NewRandomExpireCache(cache Cache, opts ...RandomExpireCacheOption) *RandomExpireCache {
	res := &RandomExpireCache{
		Cache: cache,
	}
	for _, fn := range opts {
		fn(res)
	}
	return res
}

//func (c *RandomExpirationCache) SetV1(ctx context.Context,
//	key string, val any, expiration time.Duration) error {
//	offset := rand.Intn(300)
//	expiration = expiration + time.Duration(offset) * time.Second
//	return c.Cache.Set(ctx, key, val, expiration)
//}

func (c *RandomExpireCache) Set(ctx context.Context,
	key string, val any, expiration time.Duration) error {
	expiration = expiration + c.offset()
	return c.Cache.Set(ctx, key, val, expiration)
}

// defaultExpiredFunc return a func that used to generate random time offset (range: [3s,8s)) expired
func defaultExpiredFunc() func() time.Duration {
	const size = 5
	var randTimes [size]time.Duration
	for i := range randTimes {
		randTimes[i] = time.Duration(i+3) * time.Second
	}
	// shuffle values
	for i := range randTimes {
		n := rand.Intn(size)
		randTimes[i], randTimes[n] = randTimes[n], randTimes[i]
	}
	var i uint64
	return func() time.Duration {
		return randTimes[atomic.AddUint64(&i, 1)%size]
	}
}
