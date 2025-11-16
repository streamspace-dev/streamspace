// Package cache provides Redis-based caching for StreamSpace API.
//
// This file implements the core Redis cache client with connection pooling.
//
// Purpose:
// - Provide high-performance caching for frequently accessed data
// - Reduce database load for read-heavy operations
// - Enable distributed caching across multiple API instances
// - Support atomic operations and distributed locks
//
// Features:
// - Connection pooling (25 max connections, 5 min idle)
// - Automatic retry with exponential backoff
// - Graceful fallback when Redis is unavailable (cache disabled mode)
// - JSON serialization/deserialization
// - TTL-based expiration
// - Pattern-based invalidation
// - Atomic counters and distributed locks (SetNX)
// - Statistics and monitoring (pool stats, hit/miss tracking)
//
// Cache Strategy:
//   - Get: Retrieve value, deserialize JSON
//   - Set: Serialize to JSON, store with TTL
//   - Delete: Remove single or multiple keys
//   - DeletePattern: Bulk invalidation via pattern matching
//   - SetNX: Distributed lock acquisition
//
// Implementation Details:
// - Uses go-redis client with connection pooling
// - Auto-reconnection on connection failures
// - 3 retry attempts with 8-512ms exponential backoff
// - 5-second dial timeout, 3-second read/write timeouts
// - Values stored as JSON for flexibility
//
// Thread Safety:
// - Redis client is thread-safe
// - Safe for concurrent access across goroutines
//
// Dependencies:
// - github.com/redis/go-redis/v9 for Redis client
//
// Example Usage:
//
//	// Initialize cache
//	cache, err := cache.NewCache(cache.Config{
//	    Host:     "localhost",
//	    Port:     "6379",
//	    Password: "",
//	    DB:       0,
//	    Enabled:  true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cache.Close()
//
//	// Store session in cache
//	err = cache.Set(ctx, cache.SessionKey("session-123"), session, 5*time.Minute)
//
//	// Retrieve from cache
//	var session Session
//	err = cache.Get(ctx, cache.SessionKey("session-123"), &session)
//
//	// Invalidate user's sessions
//	err = cache.DeletePattern(ctx, cache.UserPattern("user-123"))
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides caching functionality using Redis
type Cache struct {
	client *redis.Client
}

// Config holds cache configuration
type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
	Enabled  bool
}

// NewCache creates a new Redis cache client
func NewCache(config Config) (*Cache, error) {
	if !config.Enabled {
		return &Cache{client: nil}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,

		// Connection pool settings for optimal performance
		PoolSize:        25,  // Maximum number of socket connections
		MinIdleConns:    5,   // Minimum idle connections
		MaxIdleConns:    10,  // Maximum idle connections
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// Retry configuration
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &Cache{client: client}, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// IsEnabled returns whether caching is enabled
func (c *Cache) IsEnabled() bool {
	return c.client != nil
}

// Get retrieves a value from cache and unmarshals it into target
func (c *Cache) Get(ctx context.Context, key string, target interface{}) error {
	if !c.IsEnabled() {
		return fmt.Errorf("cache not enabled")
	}

	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	if err := json.Unmarshal([]byte(val), target); err != nil {
		return fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return nil
}

// Set stores a value in cache with the given TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !c.IsEnabled() {
		return nil // Silently skip if caching disabled
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key from cache
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if !c.IsEnabled() {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	return nil
}

// DeletePattern deletes all keys matching a pattern
func (c *Cache) DeletePattern(ctx context.Context, pattern string) error {
	if !c.IsEnabled() {
		return nil
	}

	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	keys := []string{}

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys with pattern %s: %w", pattern, err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// Exists checks if a key exists in cache
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if !c.IsEnabled() {
		return false, nil
	}

	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// SetNX sets a key only if it doesn't exist (for distributed locks)
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if !c.IsEnabled() {
		return false, fmt.Errorf("cache not enabled")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	set, err := c.client.SetNX(ctx, key, data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return set, nil
}

// Expire sets a TTL on an existing key
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if !c.IsEnabled() {
		return nil
	}

	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set expiration on key %s: %w", key, err)
	}

	return nil
}

// TTL returns the remaining TTL for a key
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if !c.IsEnabled() {
		return 0, fmt.Errorf("cache not enabled")
	}

	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}

	return ttl, nil
}

// Increment atomically increments a counter
func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	if !c.IsEnabled() {
		return 0, fmt.Errorf("cache not enabled")
	}

	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}

	return val, nil
}

// IncrementBy atomically increments a counter by a specific amount
func (c *Cache) IncrementBy(ctx context.Context, key string, amount int64) (int64, error) {
	if !c.IsEnabled() {
		return 0, fmt.Errorf("cache not enabled")
	}

	val, err := c.client.IncrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}

	return val, nil
}

// FlushAll clears all keys from cache (use with caution!)
func (c *Cache) FlushAll(ctx context.Context) error {
	if !c.IsEnabled() {
		return nil
	}

	if err := c.client.FlushAll(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}

	return nil
}

// GetStats returns cache statistics
func (c *Cache) GetStats(ctx context.Context) (map[string]string, error) {
	if !c.IsEnabled() {
		return map[string]string{"enabled": "false"}, nil
	}

	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}

	// Also get connection pool stats
	poolStats := c.client.PoolStats()

	stats := map[string]string{
		"enabled":        "true",
		"info":           info,
		"hits":           fmt.Sprintf("%d", poolStats.Hits),
		"misses":         fmt.Sprintf("%d", poolStats.Misses),
		"total_conns":    fmt.Sprintf("%d", poolStats.TotalConns),
		"idle_conns":     fmt.Sprintf("%d", poolStats.IdleConns),
		"stale_conns":    fmt.Sprintf("%d", poolStats.StaleConns),
	}

	return stats, nil
}
