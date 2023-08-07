package cache

import (
	"cache/internal/errs"
	"context"
	"errors"
	"fmt"
	"time"
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

func (c *WriteDeleteCache) Set(ctx context.Context, key string, val any) error {
	err := c.storeFunc(ctx, key, val)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		wrapErr := errors.New(fmt.Sprintf("%s, %s", err.Error(), fmt.Sprintf("key: %s, val: %v", key, val)))
		return errs.ErrStoreFailed(wrapErr)
	}
	return c.Cache.Delete(ctx, key)
}

type WriteDoubleDeleteCache struct {
	Cache
	interval  time.Duration
	timeout   time.Duration
	storeFunc func(ctx context.Context, key string, val any) error
}

type WriteDoubleDeleteCacheOption func(c *WriteDoubleDeleteCache)

func NewWriteDoubleDeleteCache(cache Cache, interval, timeout time.Duration,
	fn func(ctx context.Context, key string, val any) error) (*WriteDoubleDeleteCache, error) {
	if fn == nil {
		return nil, errs.ErrStoreFuncRequired
	}
	if cache == nil {
		return nil, errs.ErrCacheRequired
	}

	return &WriteDoubleDeleteCache{
		Cache:     cache,
		interval:  interval,
		timeout:   timeout,
		storeFunc: fn,
	}, nil
}

func (c *WriteDoubleDeleteCache) Set(ctx context.Context, key string, val any) error {
	err := c.storeFunc(ctx, key, val)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		wrapErr := errors.New(fmt.Sprintf("%s, %s", err.Error(), fmt.Sprintf("key: %s, val: %v", key, val)))
		return errs.ErrStoreFailed(wrapErr)
	}
	time.AfterFunc(c.interval, func() {
		rCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
		_ = c.Cache.Delete(rCtx, key)
		cancel()
	})
	return c.Cache.Delete(ctx, key)
}
