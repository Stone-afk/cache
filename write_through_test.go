package cache

import (
	"context"
	"fmt"
	"time"
)

func ExampleNewWriteThroughCache() {
	c, err := NewLocalCache()
	if err != nil {
		panic(err)
	}
	wtc, err := NewWriteThroughCache(c, func(ctx context.Context, key string, val any) error {
		fmt.Printf("write data to somewhere key %s, val %v \n", key, val)
		return nil
	})
	if err != nil {
		panic(err)
	}
	err = wtc.Set(context.Background(),
		"/biz/user/id=1", "I am user 1", time.Minute)
	if err != nil {
		panic(err)
	}
	// Output:
	// write data to somewhere key /biz/user/id=1, val I am user 1
}
