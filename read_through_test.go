package cache

import (
	"context"
	"fmt"
	"time"
)

func ExampleReadThroughCache() {
	c, err := NewLocalCache()
	if err != nil {
		panic(err)
	}
	rc, err := NewReadThroughCache(c,
		// expiration, same as the expiration of key
		time.Minute,
		// load func, how to load data if the key is absent.
		// in general, you should load data from database.
		func(ctx context.Context, key string) (any, error) {
			return fmt.Sprintf("hello, %s", key), nil
		})
	if err != nil {
		panic(err)
	}

	val, err := rc.Get(context.Background(), "Beego")
	if err != nil {
		panic(err)
	}
	fmt.Print(val)

	// Output:
	// hello, Beego
}
