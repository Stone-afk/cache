package rlock

import (
	"cache/internal/errs"
	"cache/redis-lock/mocks"
	"context"
	"errors"
	"github.com/go-redis/redis/v9"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestClient_LockV1(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name       string
		mock       func() redis.Cmdable
		key        string
		expiration time.Duration
		timeout    time.Duration
		wantLock   *LockV1
		wantErr    string
		retry      RetryStrategy
	}{
		{
			name: "locked",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background(), nil)
				res.SetVal("OK")
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"locked-key"}, gomock.Any()).
					Return(res)
				return cmdable
			},
			key:        "locked-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Second, maxCnt: 1},
			timeout:    time.Second,
			wantLock: &LockV1{
				key:        "locked-key",
				expiration: time.Minute,
			},
		},
		{
			name: "not retryable",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background(), nil)
				res.SetErr(errors.New("network error"))
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"locked-key"}, gomock.Any()).
					Return(res)
				return cmdable
			},
			key:        "locked-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Second, maxCnt: 1},
			timeout:    time.Second,
			wantErr:    "network error",
		},
		{
			name: "retry over times",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				first := redis.NewCmd(context.Background(), nil)
				first.SetErr(context.DeadlineExceeded)
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"retry-key"}, gomock.Any()).
					Times(3).Return(first)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, maxCnt: 2},
			timeout:    time.Second,
			wantErr:    "rlock: 重试机会耗尽，最后一次重试错误: context deadline exceeded",
		},
		{
			name: "retry over times-lock holded",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				first := redis.NewCmd(context.Background(), nil)
				//first.Set
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"retry-key"}, gomock.Any()).
					Times(3).Return(first)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, maxCnt: 2},
			timeout:    time.Second,
			wantErr:    "rlock: 重试机会耗尽，锁被人持有: rlock: 抢锁失败",
		},
		{
			name: "retry and success",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				first := redis.NewCmd(context.Background(), nil)
				first.SetVal("")
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"retry-key"}, gomock.Any()).
					Times(2).Return(first)
				second := redis.NewCmd(context.Background(), nil)
				second.SetVal("OK")
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"retry-key"}, gomock.Any()).
					Return(second)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, maxCnt: 3},
			timeout:    time.Second,
			wantLock: &LockV1{
				key:        "retry-key",
				expiration: time.Minute,
			},
		},
		{
			name: "retry but timeout",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				first := redis.NewCmd(context.Background(), nil)
				first.SetVal("")
				cmdable.EXPECT().Eval(gomock.Any(), luaLock, []string{"retry-key"}, gomock.Any()).
					Times(2).Return(first)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond * 550, maxCnt: 2},
			timeout:    time.Second,
			wantErr:    "context deadline exceeded",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRedisCmd := tc.mock()
			client := NewClient(mockRedisCmd)
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			l, err := client.Lock(ctx, tc.key, tc.expiration, tc.timeout, tc.retry)
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
				return
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, mockRedisCmd, l.client)
			assert.Equal(t, tc.key, l.key)
			assert.Equal(t, tc.expiration, l.expiration)
			assert.NotEmpty(t, l.value)

		})
	}
}

func TestLock_UnlockV1(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name    string
		mock    func() redis.Cmdable
		wantErr error
	}{
		{
			// 解锁成功
			name: "unlocked",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(1))
				cmdable.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).
					Return(res)
				return cmdable
			},
		},
		{
			// 解锁失败，因为网络问题
			name: "network error",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background())
				res.SetErr(errors.New("network error"))
				cmdable.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).
					Return(res)
				return cmdable
			},
			wantErr: errors.New("network error"),
		},
		{
			// 解锁失败，锁已经过期，或者被人删了
			// 或者是别人的锁
			name: "lock not exist",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(0))
				cmdable.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).Return(res)
				return cmdable
			},
			wantErr: errs.ErrLockNotHold,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rdb := tc.mock()
			c := NewClientV1(rdb)
			go func() {
				lock := &LockV1{
					client:       rdb,
					key:          "auto-refreshed",
					value:        "123",
					expiration:   time.Minute,
					unlockSignal: make(chan struct{}, 1),
				}
				c.releaseLock(lock)
			}()

			l := c.getLock("mock-key", "mock value", time.Minute)
			err := l.Unlock(context.Background())
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestClient_SingleflightLockV1(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rdb := mocks.NewMockCmdable(ctrl)
	cmd := redis.NewCmd(context.Background())
	cmd.SetVal("OK")
	rdb.EXPECT().Eval(gomock.Any(), luaLock, gomock.Any(), gomock.Any()).
		Return(cmd)
	client := NewClientV1(rdb)
	go func() {
		lock := &LockV1{
			client:       rdb,
			key:          "auto-refreshed",
			value:        "123",
			expiration:   time.Minute,
			unlockSignal: make(chan struct{}, 1),
		}
		client.releaseLock(lock)
	}()
	// TODO 并发测试
	_, err := client.SingleflightLock(
		context.Background(),
		"key1",
		time.Minute,
		time.Second,
		&FixIntervalRetry{
			Interval: time.Millisecond,
			maxCnt:   3,
		})
	require.NoError(t, err)
}

func TestLock_AutoRefreshV1(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name         string
		unlockTiming time.Duration
		lock         func() *LockV1
		interval     time.Duration
		timeout      time.Duration
		wantErr      error
	}{
		{
			name:         "auto refresh success",
			interval:     time.Millisecond * 100,
			unlockTiming: time.Second,
			timeout:      time.Second * 2,
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				cmd1 := redis.NewCmd(context.Background(), nil)
				cmd1.SetVal(int64(1))
				rdb.EXPECT().Eval(
					gomock.Any(), luaRefresh, []string{"auto-refreshed"}, []any{"123", float64(60)}).
					AnyTimes().Return(cmd1)
				cmd2 := redis.NewCmd(context.Background())
				cmd2.SetVal(int64(1))
				rdb.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).
					Return(cmd2)
				return &LockV1{
					client:       rdb,
					key:          "auto-refreshed",
					value:        "123",
					expiration:   time.Minute,
					unlockSignal: make(chan struct{}, 1),
				}
			},
		},
		{
			name:         "auto refresh failed",
			interval:     time.Millisecond * 100,
			timeout:      time.Second * 2,
			unlockTiming: time.Second,
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				cmd1 := redis.NewCmd(context.Background(), nil)
				cmd1.SetErr(errors.New("network error"))
				rdb.EXPECT().Eval(
					gomock.Any(), luaRefresh, []string{"auto-refreshed"}, []any{"123", float64(60)}).
					AnyTimes().Return(cmd1)
				cmd2 := redis.NewCmd(context.Background())
				cmd2.SetVal(int64(1))
				rdb.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).
					Return(cmd2)
				return &LockV1{
					client:       rdb,
					key:          "auto-refreshed",
					value:        "123",
					expiration:   time.Minute,
					unlockSignal: make(chan struct{}, 1),
				}
			},
			wantErr: errors.New("network error"),
		},
		{
			name:         "auto refresh timeout",
			interval:     time.Millisecond * 100,
			unlockTiming: time.Second * 1,
			timeout:      time.Second * 2,
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				cmd1 := redis.NewCmd(context.Background(), nil)
				cmd1.SetErr(context.DeadlineExceeded)
				rdb.EXPECT().Eval(gomock.Any(), luaRefresh, []string{"auto-refreshed"}, []any{"123", float64(60)}).
					Return(cmd1)

				cmd2 := redis.NewCmd(context.Background(), nil)
				cmd2.SetVal(int64(1))
				rdb.EXPECT().Eval(gomock.Any(), luaRefresh, []string{"auto-refreshed"}, []any{"123", float64(60)}).
					AnyTimes().Return(cmd2)

				cmd3 := redis.NewCmd(context.Background())
				cmd3.SetVal(int64(1))
				rdb.EXPECT().Eval(gomock.Any(), luaUnlock, gomock.Any(), gomock.Any()).
					Return(cmd3)

				return &LockV1{
					client:       rdb,
					key:          "auto-refreshed",
					value:        "123",
					expiration:   time.Minute,
					unlockSignal: make(chan struct{}, 1),
				}
			},
		},
	}
	for _, tt := range testCases {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.lock()
			go func() {
				time.Sleep(tc.unlockTiming)
				err := lock.Unlock(context.Background())
				require.NoError(t, err)
			}()
			err := lock.AutoRefresh(context.Background(), tc.interval, tc.timeout)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestLock_RefreshV1(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name    string
		lock    func() *LockV1
		wantErr error
	}{
		{
			// 续约成功
			name: "refreshed",
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				cmd1 := redis.NewCmd(context.Background(), nil)
				cmd1.SetVal(int64(1))
				rdb.EXPECT().Eval(
					gomock.Any(), luaRefresh,
					[]string{"refreshed"}, []any{"123", float64(60)}).
					Return(cmd1)
				return &LockV1{
					client:     rdb,
					expiration: time.Minute,
					value:      "123",
					key:        "refreshed",
				}
			},
		},
		{
			// 刷新失败
			name: "refresh failed",
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background(), nil)
				res.SetErr(redis.Nil)
				rdb.EXPECT().Eval(gomock.Any(), luaRefresh, []string{"refreshed"}, []any{"123", float64(60)}).Return(res)
				return &LockV1{
					client:     rdb,
					expiration: time.Minute,
					value:      "123",
					key:        "refreshed",
				}
			},
			wantErr: redis.Nil,
		},
		{
			// 未持有锁
			name: "lock not hold",
			lock: func() *LockV1 {
				rdb := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmd(context.Background(), nil)
				res.SetVal(int64(0))
				rdb.EXPECT().Eval(gomock.Any(), luaRefresh, []string{"refreshed"}, []any{"123", float64(60)}).Return(res)
				return &LockV1{
					client:     rdb,
					expiration: time.Minute,
					value:      "123",
					key:        "refreshed",
				}
			},
			wantErr: errs.ErrLockNotHold,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.lock()
			err := lock.Refresh(context.Background())
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
