package cache

import (
	"cache/internal/errs"
	"context"
	"fmt"
	"go.uber.org/multierr"
	"sync"
	"time"
)

type ItemValue struct {
	val      any
	deadline time.Time
}

func (v *ItemValue) isExpire() bool {
	return !v.deadline.IsZero() && v.deadline.Before(time.Now())
}

type LocalCache struct {
	data          map[string]*ItemValue
	mutex         sync.RWMutex
	close         chan struct{}
	cycleInterval time.Duration
	closeOnce     sync.Once
	// onEvicted     func(ctx context.Context, key string, val any) error
	onEvicteds []func(ctx context.Context, key string, val any) error
}

type LocalCacheOption func(l *LocalCache)

func LocalCacheWithCycleInterval(interval time.Duration) LocalCacheOption {
	return func(l *LocalCache) {
		l.cycleInterval = interval
	}
}

//func LocalCacheWithOnEvictedsV1(onEvicted func(ctx context.Context, key string, val any) error) LocalCacheOption {
//	return func(l *LocalCache) {
//		originFunc := l.onEvicted
//		l.onEvicted = func(ctx context.Context, key string, val any) error {
//			if originFunc != nil {
//				if err := originFunc(ctx, key, val); err != nil {
//					return err
//				}
//			}
//			if err := onEvicted(ctx, key, val); err != nil {
//				return err
//			}
//			return nil
//		}
//	}
//}

func LocalCacheWithOnEvicteds(onEvicteds ...func(ctx context.Context, key string, val any) error) LocalCacheOption {
	return func(l *LocalCache) {
		if l.onEvicteds == nil {
			l.onEvicteds = onEvicteds
		} else {
			l.onEvicteds = append(l.onEvicteds, onEvicteds...)
		}
		l.onEvicteds = onEvicteds
	}
}

func NewLocalCache(opts ...LocalCacheOption) (*LocalCache, error) {
	res := &LocalCache{
		data:          make(map[string]*ItemValue, 16),
		close:         make(chan struct{}),
		cycleInterval: time.Second * 10,
	}
	for _, opt := range opts {
		opt(res)
	}
	if err := res.checkCycle(); err != nil {
		return nil, err
	}
	return res, nil
}

func (l *LocalCache) checkCycle() error {
	var err error
	go func() {
		// 间隔时间，过长则过期的缓存迟迟得不到删除
		// 过短，则频繁执行，效果不好（过期的 key 很少）
		ticker := time.NewTicker(l.cycleInterval)
		// 没有时间间隔，不断遍历
		for {
			select {
			// case now := <-ticker.C:
			case <-ticker.C:
				// 00:01:00
				cnt := 0
				l.mutex.Lock()
				ctx := context.Background()
				for k, v := range l.data {
					// 设置了过期时间，并且已经过期
					if v.isExpire() {
						if err = l.delete(ctx, k); err != nil {
							break
						}
					}
					cnt++
					if cnt >= 1000 {
						break
					}
				}
				l.mutex.Unlock()
			case <-l.close:
				return
			}
		}
	}()
	if err != nil {
		return err
	}
	return nil
}

//func (l *LocalCache) OnEvictedV1(ctx context.Context, key string, val any) error {
//	return l.onEvicted(ctx, key, val)
//}

func (l *LocalCache) OnEvicted(ctx context.Context, key string, val any) error {
	for _, onEvicted := range l.onEvicteds {
		err := onEvicted(ctx, key, val)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *LocalCache) Get(ctx context.Context, key string) (any, error) {
	l.mutex.RLock()
	itm, ok := l.data[key]
	l.mutex.RUnlock()
	if !ok {
		return nil, errs.ErrKeyNotFound
	}
	// 别的用户可能在这个阶段调用 Set 重新刷新 key 的 val，
	// 所以下面必须要进行 double check
	if itm.isExpire() {
		l.mutex.Lock()
		defer l.mutex.Unlock()
		itm, ok = l.data[key]
		if !ok {
			return nil, errs.ErrKeyNotFound
		}
		if itm.isExpire() {
			if err := l.delete(ctx, key); err != nil {
				return nil, err
			}
		}
		return nil, errs.ErrKeyExpired
	}
	return itm.val, nil
}

func (l *LocalCache) GetMulti(ctx context.Context, keys []string) (any, error) {
	rc := make([]interface{}, len(keys))
	var err error
	for idx, key := range keys {
		val, er := l.Get(context.WithValue(ctx, idx, key), key)
		if er != nil {
			err = multierr.Combine(er, fmt.Errorf("key [%s] error: %s", key, er.Error()))
			continue
		}
		rc[idx] = val
	}
	return rc, err
}

// Set(ctx, "key1", value1, time.Minute)
// 执行业务三十秒
// Set (ctx, "key1", value2, time.Minute)
// 再三十秒，第一个 time.AfterFunc 就要执行了

func (l *LocalCache) Set(ctx context.Context, key string,
	val any, expiration time.Duration) error {
	// 这个是有破绽的
	// time.AfterFunc(expiration, func() {
	// 	if l.m.Load(key).expiration
	// 	l.Delete(context.Background(), key)
	// })
	// 如果你想支持永不过期的，expiration = 0 就说明永不过期
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.data[key] = &ItemValue{
		val:      val,
		deadline: time.Now().Add(expiration),
	}
	return nil
}

func (l *LocalCache) Delete(ctx context.Context, key string) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	err := l.delete(ctx, key)
	if err != nil {
		return err
	}
	return nil
}

func (l *LocalCache) delete(ctx context.Context, key string) error {
	itm, ok := l.data[key]
	if ok {
		delete(l.data, key)
		err := l.OnEvicted(ctx, key, itm.val)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *LocalCache) LoadAndDelete(ctx context.Context, key string) (any, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	itm, ok := l.data[key]
	if !ok {
		return nil, errs.ErrKeyNotFound
	}
	if err := l.delete(ctx, key); err != nil {
		return nil, err
	}
	return itm.val, nil
}

func (l *LocalCache) Incr(ctx context.Context, key string) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	itm, ok := l.data[key]
	if !ok {
		return errs.ErrKeyNotFound
	}
	val, err := incr(itm.val)
	if err != nil {
		return err
	}
	itm.val = val
	return nil
}

func (l *LocalCache) Decr(ctx context.Context, key string) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	itm, ok := l.data[key]
	if !ok {
		return errs.ErrKeyNotFound
	}

	val, err := decr(itm.val)
	if err != nil {
		return err
	}
	itm.val = val
	return nil
}

// IsExist checks if cache exists in memory.
func (l *LocalCache) IsExist(ctx context.Context, key string) (bool, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	if v, ok := l.data[key]; ok {
		return !v.isExpire(), nil
	}
	return false, nil
}

func (l *LocalCache) ClearAll(context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.data = make(map[string]*ItemValue)
	return nil
}

// close 无缓存，调用两次 Close 呢？第二次会阻塞
// close 1 缓存，调用三次就会阻塞
// Close()

func (l *LocalCache) Close(ctx context.Context) error {
	// 这种写法，第二次调用会 panic
	// l.close <- struct{}{}
	// close(l.close)

	// 使用 select + default 防止多次 close 阻塞调用者
	// select {
	// case l.close<- struct{}{}:
	// 关闭 channel 要小心，发送数据到已经关闭的 channel 会引起 panic
	// 	close(l.close)
	// default:
	// 	// return errors.New("cache: 已经被关闭了")
	// 	return nil
	// }

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.closeOnce.Do(func() {
		l.close <- struct{}{}
		close(l.close)
	})

	if l.data != nil && len(l.data) != 0 {
		for key, itm := range l.data {
			err := l.OnEvicted(ctx, key, itm.val)
			if err != nil {
				return err
			}
		}
	}
	l.data = nil
	return nil
}
