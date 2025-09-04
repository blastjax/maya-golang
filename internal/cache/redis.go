package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/blastjax/maya-golang/internal/github"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client with application-specific methods
type RedisClient struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg *config.RedisConfig) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: rdb,
		ttl:    cfg.TTL,
	}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetUser retrieves a user from Redis cache by username
func (r *RedisClient) GetUser(ctx context.Context, username string) (*github.User, error) {
	key := fmt.Sprintf("user:%s", username)

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get user from Redis: %w", err)
	}

	var user github.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user from Redis: %w", err)
	}

	return &user, nil
}

// SetUser stores a user in Redis cache with TTL
func (r *RedisClient) SetUser(ctx context.Context, username string, user *github.User) error {
	key := fmt.Sprintf("user:%s", username)

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user for Redis: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set user in Redis: %w", err)
	}

	return nil
}

// DeleteUser removes a user from Redis cache
func (r *RedisClient) DeleteUser(ctx context.Context, username string) error {
	key := fmt.Sprintf("user:%s", username)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete user from Redis: %w", err)
	}

	return nil
}

// UpdateUserInCache updates user data in Redis if it exists in cache
func (r *RedisClient) UpdateUserInCache(ctx context.Context, username string, user *github.User) error {
	key := fmt.Sprintf("user:%s", username)

	// Check if the key exists
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check if user exists in Redis: %w", err)
	}

	// Only update if it exists in cache
	if exists > 0 {
		return r.SetUser(ctx, username, user)
	}

	return nil // No error if not in cache
}

// GetUserTTL returns the remaining TTL for a user in Redis
func (r *RedisClient) GetUserTTL(ctx context.Context, username string) (time.Duration, error) {
	key := fmt.Sprintf("user:%s", username)

	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for user in Redis: %w", err)
	}

	return ttl, nil
}

// FlushUsers removes all user-related keys from Redis (useful for testing/cleanup)
func (r *RedisClient) FlushUsers(ctx context.Context) error {
	keys, err := r.client.Keys(ctx, "user:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys from Redis: %w", err)
	}

	if len(keys) > 0 {
		if err := r.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete user keys from Redis: %w", err)
		}
	}

	return nil
}
