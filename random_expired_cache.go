package cache

import (
	"context"
	"time"
)

type RandomExpireCache struct {
	Cache
	Offset func() time.Duration
}

// RandomExpireCacheOption implement genreate random time offset expired option
type RandomExpireCacheOption func(*RandomExpireCache)

func NewRandomExpireCache(cache Cache, offset func() time.Duration) *RandomExpireCache {
	return &RandomExpireCache{
		Cache:  cache,
		Offset: offset,
	}
}

//func (c *RandomExpirationCache) SetV1(ctx context.Context,
//	key string, val any, expiration time.Duration) error {
//	offset := rand.Intn(300)
//	expiration = expiration + time.Duration(offset) * time.Second
//	return c.Cache.Set(ctx, key, val, expiration)
//}

func (c *RandomExpireCache) Set(ctx context.Context,
	key string, val any, expiration time.Duration) error {
	expiration = expiration + c.Offset()
	return c.Cache.Set(ctx, key, val, expiration)
}
