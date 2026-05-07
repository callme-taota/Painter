package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr, password string, db int) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) SetLoginSession(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, fmt.Sprintf("identity:session:%s", token), userID, ttl).Err()
}

func (s *RedisStore) GetLoginSession(ctx context.Context, token string) (string, error) {
	return s.client.Get(ctx, fmt.Sprintf("identity:session:%s", token)).Result()
}

func (s *RedisStore) DeleteLoginSession(ctx context.Context, token string) error {
	return s.client.Del(ctx, fmt.Sprintf("identity:session:%s", token)).Err()
}

func (s *RedisStore) SetLoginCode(ctx context.Context, target, code string, ttl time.Duration) error {
	return s.client.Set(ctx, fmt.Sprintf("identity:code:%s", target), code, ttl).Err()
}

func (s *RedisStore) VerifyLoginCode(ctx context.Context, target, code string) (bool, error) {
	key := fmt.Sprintf("identity:code:%s", target)
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if value != code {
		return false, nil
	}
	_ = s.client.Del(ctx, key).Err()
	return true, nil
}
