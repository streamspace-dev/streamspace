// Package leaderelection - Redis-based leader election backend
package leaderelection

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisBackend implements leader election using Redis SET NX with TTL.
//
// This backend is suitable for:
//   - Multi-host deployments (distributed agents)
//   - Production environments
//   - High availability setups
//
// How it works:
//   - Uses Redis SET key value NX EX ttl (set if not exists with expiry)
//   - Leader sets key with instance ID and TTL = LeaseDuration
//   - Leader renews key before TTL expires (every RenewDeadline)
//   - If leader fails to renew, key expires and standby can acquire
//   - Standby instances poll Redis for leadership
//
// Benefits over file backend:
//   - Works across multiple hosts
//   - Automatic lease expiration on leader failure
//   - Network-accessible (supports distributed deployments)
//
// Requirements:
//   - Redis server accessible to all agent instances
//   - Network connectivity between agents and Redis
type redisBackend struct {
	config      *LeaderElectorConfig
	redisClient *redis.Client
	lockKey     string
}

// newRedisBackend creates a new Redis-based leader election backend.
func newRedisBackend(config *LeaderElectorConfig) *redisBackend {
	lockKey := fmt.Sprintf("%s%s", config.RedisKeyPrefix, config.AgentID)

	log.Printf("[LeaderElection:Redis] Using lock key: %s", lockKey)

	return &redisBackend{
		config:      config,
		redisClient: config.RedisClient,
		lockKey:     lockKey,
	}
}

// TryAcquire attempts to acquire leadership by setting the Redis key.
//
// Uses SET key value NX EX ttl:
//   - NX: Only set if key doesn't exist
//   - EX: Set expiry time in seconds
func (rb *redisBackend) TryAcquire(ctx context.Context) (bool, error) {
	// Try to set the lock key with our instance ID
	// NX = only set if not exists, EX = set expiry
	result, err := rb.redisClient.SetNX(
		ctx,
		rb.lockKey,
		rb.config.InstanceID,
		rb.config.LeaseDuration,
	).Result()

	if err != nil {
		return false, fmt.Errorf("redis SetNX error: %w", err)
	}

	if result {
		log.Printf("[LeaderElection:Redis] Acquired leadership (key: %s, ttl: %s)",
			rb.lockKey, rb.config.LeaseDuration)
	}

	return result, nil
}

// Renew renews the leadership lease by updating the key's TTL.
//
// Only succeeds if we are the current leader (key value matches our instance ID).
func (rb *redisBackend) Renew(ctx context.Context) error {
	// Lua script to atomically check and renew:
	// 1. Check if key value matches our instance ID
	// 2. If yes, update TTL
	// 3. Return 1 if renewed, 0 if not leader
	script := redis.NewScript(`
		local key = KEYS[1]
		local instanceID = ARGV[1]
		local ttl = ARGV[2]

		local currentValue = redis.call('GET', key)
		if currentValue == instanceID then
			redis.call('EXPIRE', key, ttl)
			return 1
		else
			return 0
		end
	`)

	result, err := script.Run(
		ctx,
		rb.redisClient,
		[]string{rb.lockKey},
		rb.config.InstanceID,
		int(rb.config.LeaseDuration.Seconds()),
	).Result()

	if err != nil {
		return fmt.Errorf("redis renew error: %w", err)
	}

	// Check if we successfully renewed
	renewed, ok := result.(int64)
	if !ok || renewed != 1 {
		return fmt.Errorf("failed to renew: not the current leader")
	}

	return nil
}

// Release releases the leadership lock.
//
// Uses Lua script to atomically check and delete:
//   - Only deletes if key value matches our instance ID
//   - Prevents accidentally deleting another leader's lock
func (rb *redisBackend) Release(ctx context.Context) error {
	// Lua script to atomically check and delete:
	// 1. Check if key value matches our instance ID
	// 2. If yes, delete key
	// 3. Return 1 if deleted, 0 if not leader
	script := redis.NewScript(`
		local key = KEYS[1]
		local instanceID = ARGV[1]

		local currentValue = redis.call('GET', key)
		if currentValue == instanceID then
			redis.call('DEL', key)
			return 1
		else
			return 0
		end
	`)

	result, err := script.Run(
		ctx,
		rb.redisClient,
		[]string{rb.lockKey},
		rb.config.InstanceID,
	).Result()

	if err != nil {
		return fmt.Errorf("redis release error: %w", err)
	}

	// Check if we successfully released
	released, ok := result.(int64)
	if ok && released == 1 {
		log.Printf("[LeaderElection:Redis] Released leadership (key: %s)", rb.lockKey)
	} else {
		log.Printf("[LeaderElection:Redis] Not the leader, nothing to release")
	}

	return nil
}

// GetLeader returns the current leader's instance ID.
//
// Reads the lock key value from Redis.
func (rb *redisBackend) GetLeader(ctx context.Context) (string, error) {
	leader, err := rb.redisClient.Get(ctx, rb.lockKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, no leader
			return "", nil
		}
		return "", err
	}

	return leader, nil
}

// Close cleans up backend resources.
func (rb *redisBackend) Close() error {
	// Release leadership if we hold it
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return rb.Release(ctx)
}
