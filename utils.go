package cache

import (
	"cache/internal/errs"
	"math"
)

func incr(originVal any) (any, error) {
	switch val := originVal.(type) {
	case int:
		tmp := val + 1
		if val > 0 && tmp < 0 {
			return nil, errs.ErrIncrementOverflow
		}
		return tmp, nil
	case int32:
		if val == math.MaxInt32 {
			return nil, errs.ErrIncrementOverflow
		}
		return val + 1, nil
	case int64:
		if val == math.MaxInt64 {
			return nil, errs.ErrIncrementOverflow
		}
		return val + 1, nil
	case uint:
		tmp := val + 1
		if tmp < val {
			return nil, errs.ErrIncrementOverflow
		}
		return tmp, nil
	case uint32:
		if val == math.MaxUint32 {
			return nil, errs.ErrIncrementOverflow
		}
		return val + 1, nil
	case uint64:
		if val == math.MaxUint64 {
			return nil, errs.ErrIncrementOverflow
		}
		return val + 1, nil
	default:
		return nil, errs.ErrNotIntegerType
	}
}
