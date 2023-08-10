package cache

import (
	"cache/internal/errs"
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestWriteThoughCache_Set(t *testing.T) {
	mockDbStore := make(map[string]any)

	testCases := []struct {
		name      string
		cache     Cache
		storeFunc func(ctx context.Context, key string, val any) error
		key       string
		value     any
		wantErr   error
	}{
		{
			name:  "store key/value in db fail",
			cache: NewMemoryCache(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				return errors.New("failed")
			},
			wantErr: berror.Wrap(errors.New("failed"), PersistCacheFailed,
				fmt.Sprintf("key: %s, val: %v", "", nil)),
		},
		{
			name:  "store key/value success",
			cache: NewMemoryCache(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				mockDbStore[key] = val
				return nil
			},
			key:   "hello",
			value: "world",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWriteThroughCache(tt.cache, tt.storeFunc)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			err = w.Set(context.Background(), tt.key, tt.value, 60*time.Second)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			val, err := w.Get(context.Background(), tt.key)
			assert.Nil(t, err)
			assert.Equal(t, tt.value, val)

			vv, ok := mockDbStore[tt.key]
			assert.True(t, ok)
			assert.Equal(t, tt.value, vv)
		})
	}
}

func TestNewWriteThoughCache(t *testing.T) {
	underlyingCache, err := NewLocalCache()
	assert.Nil(t, err)
	storeFunc := func(ctx context.Context, key string, val any) error { return nil }

	type args struct {
		cache Cache
		fn    func(ctx context.Context, key string, val any) error
	}
	tests := []struct {
		name    string
		args    args
		wantRes *WriteThroughCache
		wantErr error
	}{
		{
			name: "nil cache parameters",
			args: args{
				cache: nil,
				fn:    storeFunc,
			},
			wantErr: errs.ErrCacheRequired,
		},
		{
			name: "nil storeFunc parameters",
			args: args{
				cache: underlyingCache,
				fn:    nil,
			},
			wantErr: errs.ErrStoreFuncRequired,
		},
		{
			name: "init write-though cache success",
			args: args{
				cache: underlyingCache,
				fn:    storeFunc,
			},
			wantRes: &WriteThroughCache{
				Cache:     underlyingCache,
				storeFunc: storeFunc,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWriteThroughCache(tt.args.cache, tt.args.fn)
			assert.Equal(t, tt.wantErr, err)
			if err != nil {
				return
			}
		})
	}
}

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
