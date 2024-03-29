package cache

import (
	"cache/internal/errs"
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWriteDoubleDeleteCache_Set(t *testing.T) {
	mockDbStore := make(map[string]any)

	cancels := make([]func(), 0)
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()
	timeout := time.Second * 3
	testCases := []struct {
		name        string
		cache       Cache
		storeFunc   func(ctx context.Context, key string, val any) error
		ctx         context.Context
		interval    time.Duration
		sleepSecond time.Duration
		key         string
		value       any
		wantErr     error
	}{
		{
			name:     "store key/value in db fail",
			interval: time.Second,
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				return errors.New("failed")
			},
			ctx: context.TODO(),
			wantErr: func() error {
				err := errors.New("failed")
				wrapErr := errors.New(fmt.Sprintf("%s, %s", err.Error(), fmt.Sprintf("key: hello, val: world")))
				return errs.ErrStoreFailed(wrapErr)
			}(),
			key:   "hello",
			value: "world",
		},
		{
			name:        "store key/value success",
			interval:    time.Second * 2,
			sleepSecond: time.Second * 3,
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				err = c.Set(context.Background(), "hello", "world", time.Second*2)
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				mockDbStore[key] = val
				return nil
			},
			ctx:   context.TODO(),
			key:   "hello",
			value: "world",
		},
		{
			name:        "store key/value timeout",
			interval:    time.Second * 2,
			sleepSecond: time.Second * 3,
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				err = c.Set(context.Background(), "hello", "hello", time.Second*2)
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(3 * time.Second):
					mockDbStore[key] = val
					return nil
				}
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				cancels = append(cancels, cancel)
				return ctx

			}(),
			key:   "hello",
			value: "hello",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			oc := tt.cache
			c, err := NewWriteDoubleDeleteCache(oc, tt.interval, timeout, tt.storeFunc)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			err = c.Set(tt.ctx, tt.key, tt.value)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			_, err = c.Get(tt.ctx, tt.key)
			assert.Equal(t, errs.ErrKeyNotFound, err)

			err = oc.Set(tt.ctx, tt.key, tt.value, tt.interval)
			require.NoError(t, err)

			val, err := c.Get(tt.ctx, tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.value, val)

			time.Sleep(tt.sleepSecond)

			_, err = c.Get(tt.ctx, tt.key)
			assert.Equal(t, errs.ErrKeyNotFound, err)
		})
	}
}

func TestNewWriteDoubleDeleteCache(t *testing.T) {
	underlyingCache, err := NewLocalCache()
	assert.Nil(t, err)
	storeFunc := func(ctx context.Context, key string, val any) error { return nil }

	type args struct {
		cache    Cache
		interval time.Duration
		fn       func(ctx context.Context, key string, val any) error
	}
	timeout := time.Second * 3
	tests := []struct {
		name    string
		args    args
		wantRes *WriteDoubleDeleteCache
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
				cache:    underlyingCache,
				fn:       storeFunc,
				interval: time.Second,
			},
			wantRes: &WriteDoubleDeleteCache{
				Cache:     underlyingCache,
				storeFunc: storeFunc,
				interval:  time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWriteDoubleDeleteCache(tt.args.cache, tt.args.interval, timeout, tt.args.fn)
			assert.Equal(t, tt.wantErr, err)
			if err != nil {
				return
			}
		})
	}
}

func ExampleWriteDoubleDeleteCache() {
	c, err := NewLocalCache()
	if err != nil {
		panic(err)
	}
	wtc, err := NewWriteDoubleDeleteCache(c, 1*time.Second, 3*time.Second, func(ctx context.Context, key string, val any) error {
		fmt.Printf("write data to somewhere key %s, val %v \n", key, val)
		return nil
	})
	if err != nil {
		panic(err)
	}
	err = wtc.Set(context.Background(),
		"/biz/user/id=1", "I am user 1")
	if err != nil {
		panic(err)
	}
	// Output:
	// write data to somewhere key /biz/user/id=1, val I am user 1
}

func TestWriteDeleteCache_Set(t *testing.T) {
	mockDbStore := make(map[string]any)

	cancels := make([]func(), 0)
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	testCases := []struct {
		name      string
		cache     Cache
		storeFunc func(ctx context.Context, key string, val any) error
		ctx       context.Context
		key       string
		value     any
		wantErr   error
		before    func(Cache)
		after     func()
	}{
		{
			name:  "store key/value in db fail",
			key:   "hello",
			value: "world",
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				return errors.New("failed")
			},
			ctx: context.TODO(),
			wantErr: func() error {
				err := errors.New("failed")
				wrapErr := errors.New(fmt.Sprintf("%s, %s", err.Error(), fmt.Sprintf("key: hello, val: world")))
				return errs.ErrStoreFailed(wrapErr)
			}(),
			before: func(cache Cache) {},
			after:  func() {},
		},
		{
			name: "store key/value success",
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				mockDbStore[key] = val
				return nil
			},
			ctx:   context.TODO(),
			key:   "hello",
			value: "world",
			before: func(cache Cache) {
				_ = cache.Set(context.Background(), "hello", "testVal", 10*time.Second)
			},
			after: func() {
				delete(mockDbStore, "hello")
			},
		},
		{
			name: "store key/value timeout",
			cache: func() Cache {
				c, err := NewLocalCache()
				assert.Nil(t, err)
				return c
			}(),
			storeFunc: func(ctx context.Context, key string, val any) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(3 * time.Second):
					mockDbStore[key] = val
					return nil
				}

			},
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				cancels = append(cancels, cancel)
				return ctx

			}(),
			key:   "hello",
			value: nil,
			before: func(cache Cache) {
				_ = cache.Set(context.Background(), "hello", "testVal", 10*time.Second)
			},
			after: func() {},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWriteDeleteCache(tt.cache, tt.storeFunc)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			tt.before(tt.cache)
			defer func() {
				tt.after()
			}()

			err = w.Set(tt.ctx, tt.key, tt.value)
			if err != nil {
				assert.EqualError(t, tt.wantErr, err.Error())
				return
			}

			_, err = w.Get(tt.ctx, tt.key)
			assert.Equal(t, errs.ErrKeyNotFound, err)

			vv := mockDbStore[tt.key]
			assert.Equal(t, tt.value, vv)
		})
	}
}

func TestNewWriteDeleteCache(t *testing.T) {
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
		wantRes *WriteDeleteCache
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
			wantRes: &WriteDeleteCache{
				Cache:     underlyingCache,
				storeFunc: storeFunc,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWriteDeleteCache(tt.args.cache, tt.args.fn)
			assert.Equal(t, tt.wantErr, err)
			if err != nil {
				return
			}
		})
	}
}

func ExampleNewWriteDeleteCache() {
	c, err := NewLocalCache()
	if err != nil {
		panic(err)
	}
	wtc, err := NewWriteDeleteCache(c, func(ctx context.Context, key string, val any) error {
		fmt.Printf("write data to somewhere key %s, val %v \n", key, val)
		return nil
	})
	if err != nil {
		panic(err)
	}
	err = wtc.Set(context.Background(),
		"/biz/user/id=1", "I am user 1")
	if err != nil {
		panic(err)
	}
	// Output:
	// write data to somewhere key /biz/user/id=1, val I am user 1
}
