package cache

import (
	"cache/internal/errs"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func testReadThroughCacheGet(t *testing.T, bm Cache) {
	testCases := []struct {
		name    string
		key     string
		value   string
		cache   Cache
		wantErr error
	}{
		{
			name: "Get load err",
			key:  "key0",
			cache: func() Cache {
				kvs := map[string]any{"key0": "value0"}
				db := &MockOrm{kvs: kvs}
				loadfunc := func(ctx context.Context, key string) (any, error) {
					val, er := db.Load(key)
					if er != nil {
						return nil, er
					}
					return val, nil
				}
				c, err := NewReadThroughCache(bm, 3*time.Second, loadfunc)
				assert.Nil(t, err)
				return c
			}(),
			wantErr: errs.ErrKeyNotFound,
		},
		{
			name:  "Get cache exist",
			key:   "key1",
			value: "value1",
			cache: func() Cache {
				keysMap := map[string]int{"key1": 1}
				kvs := map[string]any{"key1": "value1"}
				db := &MockOrm{keysMap: keysMap, kvs: kvs}
				loadfunc := func(ctx context.Context, key string) (any, error) {
					val, er := db.Load(key)
					if er != nil {
						return nil, er
					}
					return val, nil
				}
				c, err := NewReadThroughCache(bm, 3*time.Second, loadfunc)
				assert.Nil(t, err)
				err = c.Set(context.Background(), "key1", "value1", 3*time.Second)
				assert.Nil(t, err)
				return c
			}(),
		},
		{
			name:  "Get loadFunc exist",
			key:   "key2",
			value: "value2",
			cache: func() Cache {
				keysMap := map[string]int{"key2": 1}
				kvs := map[string]any{"key2": "value2"}
				db := &MockOrm{keysMap: keysMap, kvs: kvs}
				loadfunc := func(ctx context.Context, key string) (any, error) {
					val, er := db.Load(key)
					if er != nil {
						return nil, er
					}
					return val, nil
				}
				c, err := NewReadThroughCache(bm, 3*time.Second, loadfunc)
				assert.Nil(t, err)
				return c
			}(),
		},
	}
	_, err := NewReadThroughCache(bm, 3*time.Second, nil)
	assert.Equal(t, errs.ErrLoadFuncRequired, err)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cache
			val, err := c.Get(context.Background(), tc.key)
			if err != nil {
				assert.EqualError(t, tc.wantErr, err.Error())
				return
			}
			assert.Equal(t, tc.value, val)
		})
	}
}

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
