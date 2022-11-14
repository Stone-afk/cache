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
	"time"
)

var (
	//go:embed script/lua/unlock.lua
	luaUnlock string
	//go:embed script/lua/refresh.lua
	luaRefresh string

	//go:embed script/lua/lock.lua
	luaLock string
)

type Client struct {
	client redis.Cmdable
	// valuer 用于生成值，将来可以考虑暴露出去允许用户自定义
	valuer func() string
	g      *singleflight.Group
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
		g:      &singleflight.Group{},
		valuer: func() string {
			return uuid.New().String()
		},
	}
}

func (c *Client) SingleflightLock(ctx context.Context, key string,
	expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	flag := false
	for {
		result := c.g.DoChan(key, func() (interface{}, error) {
			flag = true
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case res := <-result:
			if flag {
				c.g.Forget(key)
				if res.Err != nil {
					return nil, res.Err
				}
				return res.Val.(*Lock), nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Lock 是尽可能重试减少加锁失败的可能
// Lock 会在超时或者锁正被人持有的时候进行重试
// 最后返回的 error 使用 errors.Is 判断，可能是：
// - context.DeadlineExceeded: Lock 整体调用超时
// - ErrFailedToPreemptLock: 超过重试次数，但是整个重试过程都没有出现错误
// - DeadlineExceeded 和 ErrFailedToPreemptLock: 超过重试次数，但是最后一次重试超时了
// 你在使用的过程中，应该注意：
// - 如果 errors.Is(err, context.DeadlineExceeded) 那么最终有没有加锁成功，谁也不知道
// - 如果 errors.Is(err, ErrFailedToPreemptLock) 说明肯定没成功，而且超过了重试次数
// - 否则，和 Redis 通信出了问题
func (c *Client) Lock(ctx context.Context, key string,
	expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	val := c.valuer()
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		lctx, cancel := context.WithTimeout(ctx, timeout)
		res, err := c.client.Eval(lctx, luaLock, []string{key}, val, expiration.Seconds()).Result()
		cancel()
		// 此错误非超时错误
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			// 非超时错误，那么基本上代表遇到了一些不可挽回的场景，所以没太大必要继续尝试了
			// 比如说 Redis server 崩了，或者 EOF 了
			return nil, err
		}
		// 加锁成功
		if res == "OK" {
			return newLock(c.client, key, val, expiration), nil
		}
		interval, ok := retry.Next()
		if !ok {
			//  errors.Is(err, context.DeadlineExceeded) 代表此错误是超时错误
			if err != nil {
				err = fmt.Errorf("最后一次重试错误: %w", err)
			} else {
				// res != 1
				err = fmt.Errorf("锁被人持有: %w", errs.ErrFailedToPreemptLock)
			}
			return nil, fmt.Errorf("rlock: 重试机会耗尽，%w", err)
		}
		if timer == nil {
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

func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	// 我怎么知道，那是我的锁
	ok, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		// 网络问题，服务器问题，或者超时，都会走过来这里
		return nil, err
	}
	if !ok {
		// 已经有人加锁了，或者刚好和人一起加锁，但是自己竞争失败了
		return nil, errs.ErrFailedToPreemptLock
	}
	return newLock(c.client, key, val, expiration), nil
}

type Lock struct {
	client       redis.Cmdable
	expiration   time.Duration
	key          string
	value        string
	unLockSignal chan struct{}
	unlockOnce   sync.Once
}

func newLock(client redis.Cmdable, key string, val string, expiration time.Duration) *Lock {
	return &Lock{
		client:       client,
		key:          key,
		value:        val,
		expiration:   expiration,
		unLockSignal: make(chan struct{}, 1),
	}
}

// Unlock 解锁
func (l *Lock) Unlock(ctx context.Context) error {
	// 要考虑，用 lua 脚本来封装检查-删除的两个步骤
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.value).Int64()
	defer func() {
		// 避免重复关闭引起 panic
		l.unlockOnce.Do(func() {
			l.unLockSignal <- struct{}{}
			close(l.unLockSignal)
		})
	}()

	// 方是 redis.Nil 的检测，说明根本没有拿到锁
	if err == redis.Nil {
		return errs.ErrLockNotHold
	}

	// 意味着可能服务器出错了，或者
	//超时了
	if err != nil {
		return err
	}

	// 意味着锁确实存在，但是却不是自己的锁
	if res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

// Refresh 续约
func (l *Lock) Refresh(ctx context.Context) error {
	// 续约续多长时间？
	res, err := l.client.Eval(
		ctx, luaRefresh, []string{l.key}, l.value, l.expiration.Seconds()).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

//func (c *Client) Do(ctx context.Context, key string, biz func()) error {
//
//	l, err := c.TryLock(ctx, key, time.Second*10)
//	if err != nil {
//		return err
//	}
//	go l.AutoRefresh(time.Second*10, time.Second*10)
//	biz()
//	err = l.Unlock(ctx)
//	if err != nil {
//		return err
//	}
//	return nil
//}

// AutoRefresh 自动续约
func (l *Lock) AutoRefresh(internal time.Duration, timeout time.Duration) error {

	// 间隔时间根据你的锁过期时间来决定
	ticker := time.NewTicker(internal)
	// 刷新超时重试 channel
	retrySignal := make(chan struct{}, 1)
	defer func() {
		ticker.Stop()
		close(retrySignal)
	}()

	// 不断续约，直到收到退出信号
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			// 执行完 Refresh 就 cancel()
			cancel()

			// 超时这里，可以继续尝试
			if errors.Is(err, context.DeadlineExceeded) {
				// 因为有两个可能的地方要写入数据，而 retrySignal
				// 容量只有一个，所以如果写不进去就说明前一次调用超时了，并且还没被处理，
				// 与此同时计时器也触发了
				select {
				case retrySignal <- struct{}{}:
				default:
				}
				continue
			}

			if err != nil {
				// 不可挽回的错误
				// 你这里要考虑中断业务执行
				return err
			}
		case <-retrySignal:
			// 收到重试信号，执行重试
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			// 执行完 Refresh 就 cancel()
			cancel()

			// 超时了
			if errors.Is(err, context.DeadlineExceeded) {
				retrySignal <- struct{}{}
				continue
			}

			if err != nil {
				return err
			}
		case <-l.unLockSignal:
			return nil
		}
	}
}

func ExampleLockRefresh() {
	// 假如说我们拿到了一个锁
	var l *Lock
	stopSingal := make(chan struct{}, 1)
	bizStopSingal := make(chan struct{}, 1)
	go func() {
		// 间隔时间根据你的锁过期时间来决定
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		retryCnt := 0
		trySingal := make(chan struct{}, 1)
		// 不断续约，直到收到退出信号
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				err := l.Refresh(ctx)
				// 执行完 Refresh 就 cancel()
				cancel()

				// error 怎么处理
				// 可能需要对 err 分类处理

				// 超时了
				if errors.Is(err, context.DeadlineExceeded) {
					// 可以重试
					// 如果一直重试失败，又怎么办？
					trySingal <- struct{}{}
					continue
				}

				if err != nil {
					// 不可挽回的错误
					// 你这里要考虑中断业务执行
					bizStopSingal <- struct{}{}
					return
				}
				retryCnt = 0
			case <-trySingal:
				retryCnt++
				// 收到重试信号，执行重试
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				err := l.Refresh(ctx)
				// 执行完 Refresh 就 cancel()
				cancel()

				// error 怎么处理
				// 可能需要对 err 分类处理

				// 超时了
				if errors.Is(err, context.DeadlineExceeded) {
					// 可以重试
					// 如果一直重试失败，又怎么办？
					if retryCnt > 10 {
						// 考虑中断业务
						// bizStopSingal <- struct{}{}
						return
					}
					trySingal <- struct{}{}
					continue
				}

				if err != nil {
					// 不可挽回的错误
					// 你这里要考虑中断业务执行
					bizStopSingal <- struct{}{}
					return
				}
				retryCnt = 0
			case <-stopSingal:
				return
			}
		}
	}()

	// 这边就是你的业务
	for {
		select {
		case <-bizStopSingal:
			// 要回滚的
			break
		default:
			// 你的业务，你的业务被拆成了好几个步骤，非常多的步骤
		}
	}

	// 业务结束
	// 这边是不是要通知不用再续约了
	stopSingal <- struct{}{}

	// Output:
	// hello world
}
