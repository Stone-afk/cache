package cache

import (
	"context"
	"time"
)

type RandomExpirationCache struct {
	Cache
	Offset func() time.Duration
}

func NewRandomExpirationCache(cache Cache, offset func() time.Duration) *RandomExpirationCache {
	return &RandomExpirationCache{
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

func (c *RandomExpirationCache) Set(ctx context.Context,
	key string, val any, expiration time.Duration) error {
	expiration = expiration + c.Offset()
	return c.Cache.Set(ctx, key, val, expiration)
}
