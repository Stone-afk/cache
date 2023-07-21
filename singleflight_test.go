package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestSingleflight_Memory_Get(t *testing.T) {
	bm, err := NewLocalCache()
	assert.Nil(t, err)

	testSingleflightCacheConcurrencyGet(t, bm)
}

func testSingleflightCacheConcurrencyGet(t *testing.T, bm Cache) {
	key, value := "key3", "value3"
	db := &MockOrm{keysMap: map[string]int{key: 1}, kvs: map[string]any{key: value}}
	c, err := NewSingleflightCache(bm, 10*time.Second,
		func(ctx context.Context, key string) (any, error) {
			val, er := db.Load(key)
			if er != nil {
				return nil, er
			}
			return val, nil
		})
	assert.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			val, err := c.Get(context.Background(), key)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, value, val)
		}()
		time.Sleep(1 * time.Millisecond)
	}
	wg.Wait()
}

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

type MockOrm struct {
	keysMap map[string]int
	kvs     map[string]any
}

func (m *MockOrm) Load(key string) (any, error) {
	_, ok := m.keysMap[key]
	if !ok {
		return nil, errors.New("the key not exist")
	}
	return m.kvs[key], nil
}
