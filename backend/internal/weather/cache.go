package weather

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrMiss — ключа нет в кэше.
var ErrMiss = errors.New("weather: cache miss")

// Cache — хранилище ответов источника (Redis; в тестах — map).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, keep time.Duration) error
}

type redisCache struct{ c *redis.Client }

// NewRedisCache — кэш в Redis; nil — без кэша, каждый раз к источнику.
func NewRedisCache(c *redis.Client) Cache {
	if c == nil {
		return nil
	}
	return redisCache{c}
}

func (r redisCache) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return data, err
}

func (r redisCache) Set(ctx context.Context, key string, value []byte, keep time.Duration) error {
	return r.c.Set(ctx, key, value, keep).Err()
}
