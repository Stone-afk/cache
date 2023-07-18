package errs

import (
	"errors"
	"fmt"
)

var (
	ErrFailedToPreemptLock = errors.New("rlock: 抢锁失败")
	// ErrLockNotHold 一般是出现在你预期你本来持有锁，结果却没有持有锁的地方
	// 比如说当你尝试释放锁的时候，可能得到这个错误
	// 这一般意味着有人绕开了 rlock 的控制，直接操作了 Redis
	ErrLockNotHold       = errors.New("rlock: 未持有锁")
	ErrKeyNotFound       = errors.New("cache: 找不到 key")
	ErrKeyExpired        = errors.New("cache: key 已经过期")
	ErrOverCapacity      = errors.New("cache: 超过缓存最大容量")
	ErrFailedToSetCache  = errors.New("cache: 设置键值对失败")
	ErrInvalidkey        = errors.New("cache: invalid key")
	ErrStoreFuncRequired = errors.New("cache: cache or storeFunc can not be nil")
)

func ErrCannotStore(err error) error {
	return fmt.Errorf("cache 无法存储数据: %w", err)
}

func ErrCannotLoad(err error) error {
	return fmt.Errorf("cache 无法加载数据: %w", err)
}
