package cache

import "context"

type WriteDeleteCache struct {
	Cache
	storeFunc func(ctx context.Context, key string, val any) error
}
