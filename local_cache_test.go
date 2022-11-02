package cache

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMaxCntCache_Get(t *testing.T) {
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

func TestMaxCntCache_Set(t *testing.T) {
	cache, err := NewLocalCache()
	if err != nil {
		t.Error(err)
	}
	cntCache := NewMaxCntCache(cache, 2)
	err = cntCache.Set(context.Background(), "key1", 123, time.Second*10)
	require.NoError(t, err)

	err = cntCache.Set(context.Background(), "key2", 456, time.Second*10)
	require.NoError(t, err)

	err = cntCache.Set(context.Background(), "key3", 789, time.Second*10)
	assert.Equal(t, errOverCapacity, err)

	err = cntCache.Delete(context.Background(), "key1")
	require.NoError(t, err)

	// 可以放进去了
	err = cntCache.Set(context.Background(), "key3", 789, time.Second*10)
	require.NoError(t, err)

}
