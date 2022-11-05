package cache

import (
	"context"
	"sync"
	"time"
)

type LocalCache struct {
	data          map[string]any
	mutex         sync.RWMutex
	close         chan struct{}
	cycleInterval time.Duration
	closeOnce     sync.Once
	onEvicted     func(ctx context.Context, key string, val any) error
	onEvicteds    []func(ctx context.Context, key string, val any) error
}

type LocalCacheOption func(l *LocalCache)

func LocalCacheWithCycleInterval(interval time.Duration) LocalCacheOption {
	return func(l *LocalCache) {
		l.cycleInterval = interval
	}
}

func LocalCacheWithOnEvicteds(
	onEvicted func(ctx context.Context, key string, val any) error) LocalCacheOption {
	return func(l *LocalCache) {
		originFunc := l.onEvicted
		l.onEvicted = func(ctx context.Context, key string, val any) error {
			if originFunc != nil {
				if err := originFunc(ctx, key, val); err != nil {
					return err
				}
			}
			if err := onEvicted(ctx, key, val); err != nil {
				return err
			}
			return nil
		}
	}
}

func LocalCacheWithOnEvictedsV1(
	onEvicteds ...func(ctx context.Context, key string, val any) error) LocalCacheOption {
	return func(l *LocalCache) {
		if l.onEvicteds == nil {
			l.onEvicteds = onEvicteds
		} else {
			l.onEvicteds = append(l.onEvicteds, onEvicteds...)
		}
		l.onEvicteds = onEvicteds
	}
}

type value struct {
	val      any
	deadline time.Time
}

func NewLocalCache(opts ...LocalCacheOption) (*LocalCache, error) {
	res := &LocalCache{
		data:          make(map[string]any, 16),
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
					itm := v.(*value)
					// 设置了过期时间，并且已经过期
					if !itm.deadline.IsZero() &&
						itm.deadline.Before(time.Now()) {
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

func (l *LocalCache) OnEvicted(ctx context.Context, key string, val any) error {
	return l.onEvicted(ctx, key, val)
}

func (l *LocalCache) OnEvictedV1(ctx context.Context, key string, val any) error {
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
	val, ok := l.data[key]
	l.mutex.RUnlock()
	if !ok {
		return nil, errKeyNotFound
	}
	// 别的用户可能在这个阶段调用 Set 重新刷新 key 的 val，
	// 所以下面必须要进行 double check
	itm := val.(*value)
	if itm.deadline.Before(time.Now()) {
		l.mutex.Lock()
		defer l.mutex.Unlock()
		val, ok = l.data[key]
		if !ok {
			return nil, errKeyNotFound
		}
		itm = val.(*value)
		if itm.deadline.Before(time.Now()) {
			if err := l.delete(ctx, key); err != nil {
				return nil, err
			}
		}
		return nil, errKeyExpired
	}
	return itm.val, nil
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
	l.data[key] = &value{
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
	val, ok := l.data[key]
	if ok {
		delete(l.data, key)
		itm := val.(*value)
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
	val, ok := l.data[key]
	itm := val.(*value)
	if !ok {
		return nil, errKeyNotFound
	}
	if err := l.delete(ctx, key); err != nil {
		return nil, err
	}
	return itm.val, nil
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
		for key, val := range l.data {
			err := l.OnEvicted(ctx, key, val.(*value).val)
			if err != nil {
				return err
			}
		}
	}
	l.data = nil
	return nil
}
