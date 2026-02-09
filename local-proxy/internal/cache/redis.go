// ./local-proxy/internal/cache/redis.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

func (c *RedisCache) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

// Set сохраняет значение с TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	fullKey := c.buildKey(key)
	return c.client.Set(ctx, fullKey, data, ttl).Err()
}

// Get получает значение
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.buildKey(key)
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return redis.Nil
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return json.Unmarshal(data, dest)
}

// Delete удаляет ключ
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := c.buildKey(key)
	return c.client.Del(ctx, fullKey).Err()
}

// Exists проверяет существование ключа
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := c.buildKey(key)
	result, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	return result > 0, nil
}

// Increment инкрементирует счетчик
func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	fullKey := c.buildKey(key)
	return c.client.Incr(ctx, fullKey).Result()
}

// SetNX устанавливает значение, если ключ не существует
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	fullKey := c.buildKey(key)
	return c.client.SetNX(ctx, fullKey, data, ttl).Result()
}

// Keys возвращает список ключей по паттерну
func (c *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	fullPattern := c.buildKey(pattern)
	return c.client.Keys(ctx, fullPattern).Result()
}

// CacheKeys - константы для ключей кеша
const (
	CacheKeyUserSessions         = "user:sessions:%s"  // user:sessions:{user_id}
	CacheKeyTicket               = "ticket:%s"         // ticket:{ticket_id}
	CacheKeyQueuePosition        = "queue:position:%s" // queue:position:{ticket_id}
	CacheKeyActiveOperators      = "operators:active"
	CacheKeyDispatcherConfig     = "dispatcher:config:%s"     // dispatcher:config:{dispatcher_id}
	CacheKeyOrchestratorResponse = "orchestrator:response:%s" // orchestrator:response:{hash}
)
