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
	ErrStoreFuncRequired = errors.New("cache: storeFunc can not be nil")
	ErrCacheRequired     = errors.New("cache: cache can not be nil")
	ErrLoadFuncRequired  = errors.New("cache: loadFunc cannot be nil")

	ErrIncrementOverflow = errors.New("cache: incr invocation will overflow")
	ErrDecrementOverflow = errors.New("cache: decr invocation will overflow")
	ErrNotIntegerType    = errors.New("cache: item val is not (u)int (u)int32 (u)int64")
)

func ErrStoreFailed(err error) error {
	return fmt.Errorf("cache 存储数据失败: %w", err)
}

func ErrLoadFailed(err error) error {
	return fmt.Errorf("cache 加载数据失败: %w", err)
}
