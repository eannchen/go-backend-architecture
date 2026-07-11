package store

import "fmt"

func redisInt(v any) (int, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("expected int64, got %T", v)
	}
	return int(n), nil
}
