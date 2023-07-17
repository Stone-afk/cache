package cache

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

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
