package cache

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLocalCache_Get(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		key     string
		wantVal any
		wantErr error
	}{
		{
			name:    "exist",
			key:     "key1",
			wantVal: 123,
		},
		{
			name:    "not exist",
			key:     "invalid",
			wantErr: errKeyNotFound,
		},
	}

	cache, err := NewLocalCache()
	if err != nil {
		t.Error(err)
	}
	err = cache.Set(context.Background(), "key1", 123, time.Second*2)
	require.NoError(t, err)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := cache.Get(context.Background(), tc.key)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tc.wantVal, val)
		})
	}
	time.Sleep(time.Second * 3)
	_, err = cache.Get(context.Background(), "key1")
	assert.Equal(t, errKeyExpired, err)
}

func TestLocalCache_checkCycle(t *testing.T) {
	f := LocalCacheWithCycleInterval(time.Second)
	c, err := NewLocalCache(f)
	assert.NoError(t, err)
	err = c.Set(context.Background(), "key1", "value1", time.Millisecond*100)
	require.NoError(t, err)
	// 以防万一
	time.Sleep(time.Second * 3)
	_, err = c.Get(context.Background(), "key1")
	assert.Equal(t, errKeyNotFound, err)
}

func TestLocalCache_Loop(t *testing.T) {
	cnt := 0
	c, err := NewLocalCache(
		LocalCacheWithCycleInterval(time.Second),
		LocalCacheWithOnEvicteds(func(ctx context.Context, key string, val any) error {
			cnt++
			return nil
		}))
	require.NoError(t, err)
	err = c.Set(context.Background(), "key1", 123, time.Second)
	require.NoError(t, err)
	time.Sleep(time.Second * 3)
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	_, ok := c.data["key1"]
	require.False(t, ok)
	require.Equal(t, 1, cnt)
}
