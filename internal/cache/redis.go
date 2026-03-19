// Package cache provides a Redis client wrapper for caching and rate limiting.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with application-specific operations.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client from the given URL.
func NewClient(ctx context.Context, redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}

	rdb := redis.NewClient(opts)

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Get retrieves a value by key. Returns empty string and false if not found.
func (c *Client) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Set stores a key-value pair with a TTL.
func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Delete removes a key.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// Exists checks if a key exists in Redis.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Incr increments a key and returns the new value. Sets TTL on first increment.
func (c *Client) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// Underlying returns the raw Redis client for advanced operations.
func (c *Client) Underlying() *redis.Client {
	return c.rdb
}
