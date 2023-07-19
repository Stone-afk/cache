package cache

import (
	"fmt"
	"time"
)

func ExampleNewSingleflightCache() {
	c := NewMemoryCache()
	c, err := NewSingleflightCache(c, time.Minute, func(ctx context.Context, key string) (any, error) {
		return fmt.Sprintf("hello, %s", key), nil
	})
	if err != nil {
		panic(err)
	}
	val, err := c.Get(context.Background(), "Beego")
	if err != nil {
		panic(err)
	}
	fmt.Print(val)
	// Output:
	// hello, Beego
}
