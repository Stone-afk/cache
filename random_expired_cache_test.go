package cache

import (
	"fmt"
	"math/rand"
	"time"
)

func ExampleNewRandomExpireCache() {
	mc := NewMemoryCache()
	// use the default strategy which will generate random time offset (range: [3s,8s)) expired
	c := NewRandomExpireCache(mc)
	// so the expiration will be [1m3s, 1m8s)
	err := c.Put(context.Background(), "hello", "world", time.Minute)
	if err != nil {
		panic(err)
	}

	c = NewRandomExpireCache(mc,
		// based on the expiration
		WithRandomExpireOffsetFunc(func() time.Duration {
			val := rand.Int31n(100)
			fmt.Printf("calculate offset")
			return time.Duration(val) * time.Second
		}))

	// so the expiration will be [1m0s, 1m100s)
	err = c.Put(context.Background(), "hello", "world", time.Minute)
	if err != nil {
		panic(err)
	}

	// Output:
	// calculate offset
}
