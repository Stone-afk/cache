package rlock

import (
	"cache/internal/errs"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"sync"
	"sync/atomic"
	"time"
)

type Option func(c *ClientV1)

type ClientV1 struct {
	client redis.Cmdable
	valuer func() string
	group  *singleflight.Group

	idleChan chan LockObj  // 空闲锁队列
	waitChan chan *lockReq // 等待锁队列

	idleTimeout time.Duration
	maxCnt      int32
	cnt         int32
	mutex       sync.Mutex
}

func NewClientV1(client redis.Cmdable, opts ...Option) *ClientV1 {
	res := &ClientV1{
		client: client,
		maxCnt: 128,
		group:  &singleflight.Group{},
		valuer: func() string {
			return uuid.New().String()
		},
		idleChan: make(chan LockObj, 16),
		waitChan: make(chan *lockReq, 128),
	}
	for _, opt := range opts {
		opt(res)
	}
	return res
}

func WithIdleTimeout(idleTimeout time.Duration) Option {
	return func(c *ClientV1) {
		c.idleTimeout = idleTimeout
	}
}
func WithMaxCnt(maxCnt int32) Option {
	return func(c *ClientV1) {
		c.maxCnt = maxCnt
	}
}

func (c *ClientV1) SingleflightLock(ctx context.Context, key string,
	expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*LockV1, error) {
	flag := false
	for {
		result := c.group.DoChan(key, func() (interface{}, error) {
			flag = true
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case res := <-result:
			if flag {
				c.group.Forget(key)
				if res.Err != nil {
					return nil, res.Err
				}
				return res.Val.(*LockV1), nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *ClientV1) Lock(ctx context.Context, key string,
	expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*LockV1, error) {
	val := c.valuer()
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		lctx, cancal := context.WithTimeout(ctx, timeout)
		res, err := c.client.Eval(lctx, luaLock, []string{key}, val, expiration).Result()
		cancal()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if res == "OK" {
			return c.getLock(key, val, expiration), nil
		}
		interval, ok := retry.Next()
		if !ok {
			if err != nil {
				err = fmt.Errorf("重试机会耗尽: %w", err)
			} else {
				err = fmt.Errorf("锁被人持有: %w", errs.ErrFailedToPreemptLock)
			}
			return nil, fmt.Errorf("加锁失败;%w", err)
		}
		if timer != nil {
			timer = time.NewTimer(interval)
		} else {
			timer.Reset(interval)
		}
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *ClientV1) getLock(key string, val string, expiration time.Duration) *LockV1 {
	// 复用别的 goroutine 解锁后的锁for
	for {
		select {
		case lockObj := <-c.idleChan:
			if lockObj.lastActive.Add(c.idleTimeout).Before(time.Now()) {
				atomic.AddInt32(&c.cnt, -1)
				continue
			}
			return lockObj.lock.ReSet(key, val, expiration)
		default:
			cnt := atomic.AddInt32(&c.cnt, 1)
			if cnt <= c.maxCnt {
				return c.newLock(key, val, expiration)
			}
			atomic.AddInt32(&c.cnt, -1)
			req := &lockReq{
				ch: make(chan LockObj, 1),
			}
			c.waitChan <- req
			lockObj := <-req.ch
			return lockObj.lock
		}
	}
}

func (c *ClientV1) newLock(key string, val string, expiration time.Duration) *LockV1 {
	// 复用别的 goroutine 解锁后的锁for
	return &LockV1{
		client:     c.client,
		key:        key,
		value:      val,
		expiration: expiration,
	}
}

func (c *ClientV1) releaseLock(lock *LockV1) {
	c.mutex.Lock()
	if len(c.waitChan) > 0 {
		req := <-c.waitChan
		c.mutex.Unlock()
		req.ch <- LockObj{lock: lock, lastActive: time.Now()}
		return
	}
	c.mutex.Unlock()
	select {
	case c.idleChan <- LockObj{lock: lock, lastActive: time.Now()}:
	default:
		atomic.AddInt32(&c.cnt, -1)
	}
	return
}

type LockV1 struct {
	client                 redis.Cmdable
	key                    string
	value                  string
	expiration             time.Duration
	unlockSignal           chan struct{}
	unlockOnce             sync.Once
	autoRefreshMaxRetryCnt int
}

func (l *LockV1) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.value).Int64()
	defer func() {
		l.unlockOnce.Do(func() {
			l.unlockSignal <- struct{}{}
			close(l.unlockSignal)
		})
	}()
	if err != nil {
		return err
	}
	if err == redis.Nil || res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

func (l *LockV1) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh,
		[]string{l.key}, l.value, l.expiration.Seconds()).Int64()
	if err != nil {
		return err
	}
	if err == redis.Nil || res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

func ExampleLockRefreshV1() {
	var l *LockV1
	stopSignal := make(chan struct{}, 1)
	retrySignal := make(chan struct{}, 1)
	bizStopSignal := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		retryCnt := 0
		for {
			select {
			case <-ticker.C:
				lctx, cancel := context.WithTimeout(context.Background(), time.Second)
				err := l.Refresh(lctx)
				cancel()
				if errors.Is(err, context.DeadlineExceeded) {
					select {
					case retrySignal <- struct{}{}:
					default:
					}
					continue
				}
				if err != nil {
					bizStopSignal <- struct{}{}
					return
				}
			case <-retrySignal:
				retryCnt++
				lctx, cancel := context.WithTimeout(context.Background(), time.Second)
				err := l.Refresh(lctx)
				cancel()

				if errors.Is(err, context.DeadlineExceeded) {
					if retryCnt >= 10 {
						bizStopSignal <- struct{}{}
						return
					}
					retrySignal <- struct{}{}
					continue
				}
				if err != nil {
					bizStopSignal <- struct{}{}
					return
				}
			case <-stopSignal:
				return
			}
		}
	}()

	// 这边就是你的业务
	for {
		select {
		case <-bizStopSignal:
			// 要回滚的
			break
		default:
			// 你的业务，你的业务被拆成了好几个步骤，非常多的步骤
		}
	}

	stopSignal <- struct{}{}
}

func (l *LockV1) AutoRefresh(ctx context.Context, interval time.Duration,
	timeout time.Duration) error {
	retrySignal := make(chan struct{}, 1)
	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		close(retrySignal)
	}()
	retryCnt := 0
	for {
		select {
		case <-ticker.C:
			lctx, cancel := context.WithTimeout(ctx, timeout)
			err := l.Refresh(lctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				select {
				case retrySignal <- struct{}{}:
				default:
				}
				continue
			}
			if err != nil {
				return err
			}
		case <-retrySignal:
			retryCnt++
			lctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err := l.Refresh(lctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				if retryCnt >= l.autoRefreshMaxRetryCnt {
					return fmt.Errorf("rlock: 重试机会耗尽，%w", err)
				}
				retrySignal <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlockSignal:
			return nil
		}
	}
}

func (l *LockV1) ReSet(key string, val string, expiration time.Duration) *LockV1 {
	l.key = key
	l.value = val
	l.expiration = expiration
	return l
}

type LockObj struct {
	lock       *LockV1
	lastActive time.Time
}

type lockReq struct {
	ch chan LockObj
}
