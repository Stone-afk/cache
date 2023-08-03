package cache

import (
	"cache/internal/errs"
	"context"
)

type WriteDeleteCache struct {
	Cache
	storeFunc func(ctx context.Context, key string, val any) error
}

func NewWriteDeleteCache(cache Cache, fn func(ctx context.Context, key string, val any) error) (*WriteDeleteCache, error) {
	if fn == nil {
		return nil, errs.ErrStoreFuncRequired
	}
	if cache == nil {
		return nil, errs.ErrCacheRequired
	}

	return &WriteDeleteCache{
		Cache:     cache,
		storeFunc: fn,
	}, nil
}