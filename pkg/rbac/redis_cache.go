//go:build redis

package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RedisClient is a minimal interface needed by RedisCache.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	FlushByPrefix(ctx context.Context, prefix string) error
}

type redisCache struct {
	client RedisClient
	ttl    time.Duration
	prefix string
}

// NewRedisCache creates a new Redis-backed permission cache.
func NewRedisCache(client RedisClient, ttl time.Duration, prefix string) PermissionCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if prefix == "" {
		prefix = "rbac"
	}
	return &redisCache{
		client: client,
		ttl:    ttl,
		prefix: prefix,
	}
}

func (c *redisCache) key(roleName string) string {
	return fmt.Sprintf("%s:role:%s", c.prefix, strings.ToLower(strings.TrimSpace(roleName)))
}

func (c *redisCache) Get(roleName string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	val, err := c.client.Get(ctx, c.key(roleName))
	if err != nil || val == "" {
		return nil, false
	}

	var perms []string
	if err := json.Unmarshal([]byte(val), &perms); err != nil {
		return nil, false
	}
	return perms, true
}

func (c *redisCache) Set(roleName string, permissions []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := json.Marshal(permissions)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, c.key(roleName), string(data), c.ttl)
}

func (c *redisCache) Invalidate(roleName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = c.client.Del(ctx, c.key(roleName))
}

func (c *redisCache) InvalidateAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = c.client.FlushByPrefix(ctx, fmt.Sprintf("%s:role:", c.prefix))
}
