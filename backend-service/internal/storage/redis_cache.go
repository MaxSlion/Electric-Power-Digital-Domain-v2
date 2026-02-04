package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr, password string, db int) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,           // Connection pool for high concurrency
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	return &RedisCache{client: client}
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, payload, ttl).Err()
}

func (r *RedisCache) GetJSON(ctx context.Context, key string, out any) error {
	payload, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Incr atomically increments a key and sets expiry if it's new
func (r *RedisCache) Incr(ctx context.Context, key string, ttl time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SetNX sets a key only if it doesn't exist (for distributed locking)
func (r *RedisCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	return r.client.SetNX(ctx, key, payload, ttl).Result()
}

func (r *RedisCache) Publish(ctx context.Context, channel string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.client.Publish(ctx, channel, b).Err()
}

// Subscribe subscribes to a channel and returns a channel for messages
func (r *RedisCache) Subscribe(ctx context.Context, channel string) (<-chan *redis.Message, func()) {
	sub := r.client.Subscribe(ctx, channel)
	return sub.Channel(), func() { _ = sub.Close() }
}

// Keys returns all keys matching a pattern (use sparingly)
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}
