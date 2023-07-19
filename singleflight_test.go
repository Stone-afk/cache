package cache

import (
	"context"
	"fmt"
	"time"
)

func ExampleNewSingleflightCache() {
	c, _ := NewLocalCache()
	s, err := NewSingleflightCache(c, time.Minute, func(ctx context.Context, key string) (any, error) {
		return fmt.Sprintf("hello, %s", key), nil
	})
	if err != nil {
		panic(err)
	}
	val, err := s.Get(context.Background(), "cache")
	if err != nil {
		panic(err)
	}
	fmt.Print(val)
	// Output:
	// hello, cache
}
