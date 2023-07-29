package cache

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func TestRandomExpireCache(t *testing.T) {
	bm, err := NewLocalCache()
	assert.Nil(t, err)

	c := NewRandomExpireCache(bm)
	// should not be nil
	assert.NotNil(t, c.offset)

	timeoutDuration := 3 * time.Second

	if err = c.Set(context.Background(), "Leon Ding", 22, timeoutDuration); err != nil {
		t.Error("set Error", err)
	}

	// testing random expire cache
	time.Sleep(timeoutDuration + 3 + time.Second)

	//if res, _ := c.IsExist(context.Background(), "Leon Ding"); !res {
	//	t.Error("check err")
	//}
}

func TestWithRandomExpireOffsetFunc(t *testing.T) {
	bm, err := NewLocalCache()
	assert.Nil(t, err)

	magic := -time.Duration(rand.Int())
	c := NewRandomExpireCache(bm, WithRandomExpireOffsetFunc(func() time.Duration {
		return magic
	}))
	// offset should return the magic value
	assert.Equal(t, magic, c.offset())
}

func ExampleNewRandomExpireCache() {
	mc, err := NewLocalCache()
	if err != nil {
		panic(err)
	}
	// use the default strategy which will generate random time offset (range: [3s,8s)) expired
	c := NewRandomExpireCache(mc)
	// so the expiration will be [1m3s, 1m8s)
	err = c.Set(context.Background(), "hello", "world", time.Minute)
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
	err = c.Set(context.Background(), "hello", "world", time.Minute)
	if err != nil {
		panic(err)
	}

	// Output:
	// calculate offset
}
