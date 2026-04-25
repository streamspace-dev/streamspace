package leaderelection

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisBackend_New tests creating a new Redis backend
func TestRedisBackend_New(t *testing.T) {
	// Create a mock Redis client (will not actually connect)
	mockClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	config := &LeaderElectorConfig{
		AgentID:        "test-agent",
		InstanceID:     "instance-1",
		RedisClient:    mockClient,
		RedisKeyPrefix: "test:leader:",
		LeaseDuration:  15 * time.Second,
	}

	backend := newRedisBackend(config)

	if backend == nil {
		t.Fatal("backend is nil")
	}

	expectedKey := "test:leader:test-agent"
	if backend.lockKey != expectedKey {
		t.Errorf("lockKey = %v, want %v", backend.lockKey, expectedKey)
	}

	if backend.redisClient != mockClient {
		t.Error("redisClient not set correctly")
	}
}

// TestRedisBackend_LockKeyFormat tests that lock key is formatted correctly
func TestRedisBackend_LockKeyFormat(t *testing.T) {
	tests := []struct {
		name           string
		agentID        string
		redisKeyPrefix string
		expectedKey    string
	}{
		{
			name:           "default prefix",
			agentID:        "docker-agent-1",
			redisKeyPrefix: "streamspace:agent:leader:",
			expectedKey:    "streamspace:agent:leader:docker-agent-1",
		},
		{
			name:           "custom prefix",
			agentID:        "agent-xyz",
			redisKeyPrefix: "custom:prefix:",
			expectedKey:    "custom:prefix:agent-xyz",
		},
		{
			name:           "no trailing colon in prefix",
			agentID:        "agent-123",
			redisKeyPrefix: "myprefix",
			expectedKey:    "myprefixagent-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

			config := &LeaderElectorConfig{
				AgentID:        tt.agentID,
				InstanceID:     "instance-1",
				RedisClient:    mockClient,
				RedisKeyPrefix: tt.redisKeyPrefix,
			}

			backend := newRedisBackend(config)

			if backend.lockKey != tt.expectedKey {
				t.Errorf("lockKey = %v, want %v", backend.lockKey, tt.expectedKey)
			}
		})
	}
}

// TestRedisBackend_TryAcquire_Integration tests acquiring leadership with real Redis
// Note: This test requires a real Redis instance and is skipped by default
func TestRedisBackend_TryAcquire_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use separate DB for tests
	})

	ctx := context.Background()

	// Test if Redis is available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Cleanup at end
	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	t.Run("acquire lock successfully", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)
		defer backend.Close()

		acquired, err := backend.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}

		if !acquired {
			t.Error("TryAcquire() = false, want true")
		}

		// Verify key exists in Redis
		val, err := client.Get(ctx, backend.lockKey).Result()
		if err != nil {
			t.Fatalf("Failed to get key from Redis: %v", err)
		}

		if val != config.InstanceID {
			t.Errorf("Key value = %v, want %v", val, config.InstanceID)
		}

		// Verify TTL is set
		ttl, err := client.TTL(ctx, backend.lockKey).Result()
		if err != nil {
			t.Fatalf("Failed to get TTL: %v", err)
		}

		if ttl <= 0 || ttl > config.LeaseDuration {
			t.Errorf("TTL = %v, expected 0 < ttl <= %v", ttl, config.LeaseDuration)
		}
	})

	t.Run("second instance cannot acquire", func(t *testing.T) {
		// First instance
		config1 := &LeaderElectorConfig{
			AgentID:        "test-agent-contention",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend1 := newRedisBackend(config1)
		defer backend1.Close()

		acquired1, err := backend1.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired1 {
			t.Fatal("First instance should acquire lock")
		}

		// Second instance tries to acquire same lock
		config2 := &LeaderElectorConfig{
			AgentID:        "test-agent-contention",
			InstanceID:     "instance-2",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend2 := newRedisBackend(config2)
		defer backend2.Close()

		acquired2, err := backend2.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}

		if acquired2 {
			t.Error("Second instance should not acquire lock")
		}
	})
}

// TestRedisBackend_Renew_Integration tests renewing leadership
func TestRedisBackend_Renew_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	t.Run("renew without lock fails", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-renew",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)
		defer backend.Close()

		// Try to renew without acquiring first
		err := backend.Renew(ctx)
		if err == nil {
			t.Error("Renew() error = nil, want error when not holding lock")
		}
	})

	t.Run("renew with lock succeeds", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-renew-success",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  3 * time.Second,
		}

		backend := newRedisBackend(config)
		defer backend.Close()

		// Acquire lock first
		acquired, err := backend.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Wait a bit
		time.Sleep(500 * time.Millisecond)

		// Get TTL before renew
		ttlBefore, err := client.TTL(ctx, backend.lockKey).Result()
		if err != nil {
			t.Fatalf("Failed to get TTL: %v", err)
		}

		// Renew lock
		err = backend.Renew(ctx)
		if err != nil {
			t.Errorf("Renew() error = %v", err)
		}

		// Get TTL after renew
		ttlAfter, err := client.TTL(ctx, backend.lockKey).Result()
		if err != nil {
			t.Fatalf("Failed to get TTL: %v", err)
		}

		// TTL should be refreshed (close to LeaseDuration)
		if ttlAfter <= ttlBefore {
			t.Errorf("TTL not refreshed: before=%v, after=%v", ttlBefore, ttlAfter)
		}
	})

	t.Run("renew fails if not leader", func(t *testing.T) {
		config1 := &LeaderElectorConfig{
			AgentID:        "test-agent-renew-not-leader",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend1 := newRedisBackend(config1)
		defer backend1.Close()

		// First instance acquires lock
		acquired, err := backend1.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock with first instance")
		}

		// Second instance tries to renew
		config2 := &LeaderElectorConfig{
			AgentID:        "test-agent-renew-not-leader",
			InstanceID:     "instance-2",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend2 := newRedisBackend(config2)
		defer backend2.Close()

		// Renew should fail because backend2 is not the leader
		err = backend2.Renew(ctx)
		if err == nil {
			t.Error("Renew() should fail when not the leader")
		}
	})
}

// TestRedisBackend_Release_Integration tests releasing leadership
func TestRedisBackend_Release_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	t.Run("release without lock is safe", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-release",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)

		// Release without acquiring should not error
		err := backend.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v, want nil", err)
		}
	})

	t.Run("release after acquire works", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-release-after-acquire",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)

		// Acquire lock
		acquired, err := backend.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Verify key exists
		exists, _ := client.Exists(ctx, backend.lockKey).Result()
		if exists != 1 {
			t.Fatal("Key should exist after acquire")
		}

		// Release lock
		err = backend.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v", err)
		}

		// Verify key is deleted
		exists, _ = client.Exists(ctx, backend.lockKey).Result()
		if exists != 0 {
			t.Error("Key should be deleted after release")
		}

		// Another instance should be able to acquire now
		config2 := &LeaderElectorConfig{
			AgentID:        "test-agent-release-after-acquire",
			InstanceID:     "instance-2",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend2 := newRedisBackend(config2)
		defer backend2.Close()

		acquired2, err := backend2.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() after release error = %v", err)
		}
		if !acquired2 {
			t.Error("Should be able to acquire lock after release")
		}
	})

	t.Run("release only deletes own lock", func(t *testing.T) {
		// First instance acquires lock
		config1 := &LeaderElectorConfig{
			AgentID:        "test-agent-release-own",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend1 := newRedisBackend(config1)
		defer backend1.Close()

		acquired, err := backend1.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock with first instance")
		}

		// Second instance tries to release (should not delete first instance's lock)
		config2 := &LeaderElectorConfig{
			AgentID:        "test-agent-release-own",
			InstanceID:     "instance-2",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend2 := newRedisBackend(config2)

		// Release should succeed (no-op)
		err = backend2.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v", err)
		}

		// First instance's lock should still exist
		val, err := client.Get(ctx, backend1.lockKey).Result()
		if err != nil {
			t.Fatal("First instance's lock should still exist")
		}

		if val != config1.InstanceID {
			t.Errorf("Key value = %v, want %v", val, config1.InstanceID)
		}
	})
}

// TestRedisBackend_GetLeader_Integration tests getting current leader
func TestRedisBackend_GetLeader_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	t.Run("no leader initially", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-getleader",
			InstanceID:     "instance-1",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)
		defer backend.Close()

		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Errorf("GetLeader() error = %v", err)
		}
		if leader != "" {
			t.Errorf("GetLeader() = %v, want empty (no leader yet)", leader)
		}
	})

	t.Run("returns leader after acquire", func(t *testing.T) {
		config := &LeaderElectorConfig{
			AgentID:        "test-agent-getleader-acquire",
			InstanceID:     "instance-123",
			RedisClient:    client,
			RedisKeyPrefix: "test:leader:",
			LeaseDuration:  5 * time.Second,
		}

		backend := newRedisBackend(config)
		defer backend.Close()

		// Acquire lock
		acquired, err := backend.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Get leader
		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Errorf("GetLeader() error = %v", err)
		}

		if leader != config.InstanceID {
			t.Errorf("GetLeader() = %v, want %v", leader, config.InstanceID)
		}
	})
}

// TestRedisBackend_Close_Integration tests closing the backend
func TestRedisBackend_Close_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	config := &LeaderElectorConfig{
		AgentID:        "test-agent-close",
		InstanceID:     "instance-1",
		RedisClient:    client,
		RedisKeyPrefix: "test:leader:",
		LeaseDuration:  5 * time.Second,
	}

	backend := newRedisBackend(config)

	// Acquire lock
	acquired, err := backend.TryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// Close should release the lock
	err = backend.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify lock was released
	exists, _ := client.Exists(ctx, backend.lockKey).Result()
	if exists != 0 {
		t.Error("Lock should be released after Close()")
	}
}

// TestRedisBackend_TTLExpiration_Integration tests that lock expires after TTL
func TestRedisBackend_TTLExpiration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	defer func() {
		client.FlushDB(ctx)
		client.Close()
	}()

	config := &LeaderElectorConfig{
		AgentID:        "test-agent-ttl",
		InstanceID:     "instance-1",
		RedisClient:    client,
		RedisKeyPrefix: "test:leader:",
		LeaseDuration:  1 * time.Second, // Short TTL for testing
	}

	backend := newRedisBackend(config)
	defer backend.Close()

	// Acquire lock
	acquired, err := backend.TryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// Verify lock exists
	exists, _ := client.Exists(ctx, backend.lockKey).Result()
	if exists != 1 {
		t.Fatal("Lock should exist after acquire")
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	// Verify lock has expired
	exists, _ = client.Exists(ctx, backend.lockKey).Result()
	if exists != 0 {
		t.Error("Lock should expire after TTL")
	}

	// Another instance should be able to acquire
	config2 := &LeaderElectorConfig{
		AgentID:        "test-agent-ttl",
		InstanceID:     "instance-2",
		RedisClient:    client,
		RedisKeyPrefix: "test:leader:",
		LeaseDuration:  5 * time.Second,
	}

	backend2 := newRedisBackend(config2)
	defer backend2.Close()

	acquired2, err := backend2.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("TryAcquire() after expiration error = %v", err)
	}
	if !acquired2 {
		t.Error("Should be able to acquire lock after expiration")
	}
}
