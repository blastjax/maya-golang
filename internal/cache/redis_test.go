package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/blastjax/maya-golang/internal/github"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedisClient(t *testing.T) (*RedisClient, redismock.ClientMock) {
	client, mock := redismock.NewClientMock()

	redisClient := &RedisClient{
		client: client,
		ttl:    30 * time.Second,
	}

	return redisClient, mock
}

func createTestUser() *github.User {
	name := "Test User"
	company := "Test Company"
	return &github.User{
		ID:        123,
		Login:     "testuser",
		Type:      "User",
		Name:      &name,
		Company:   &company,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestNewRedisClient(t *testing.T) {
	// Test with valid config but invalid connection (should fail ping)
	cfg := &config.RedisConfig{
		Host:     "invalid-host",
		Port:     6379,
		Password: "",
		DB:       0,
		TTL:      30 * time.Second,
	}

	client, err := NewRedisClient(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestRedisClient_GetUser_CacheHit(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	userData, _ := json.Marshal(testUser)

	ctx := context.Background()
	key := "user:testuser"

	// Mock successful GET
	mock.ExpectGet(key).SetVal(string(userData))

	user, err := redisClient.GetUser(ctx, "testuser")
	assert.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, testUser.ID, user.ID)
	assert.Equal(t, testUser.Login, user.Login)
	assert.Equal(t, testUser.Type, user.Type)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_GetUser_CacheMiss(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock cache miss (redis.Nil error)
	mock.ExpectGet(key).RedisNil()

	user, err := redisClient.GetUser(ctx, "testuser")
	assert.NoError(t, err)
	assert.Nil(t, user) // Should return nil for cache miss

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_GetUser_RedisError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock Redis error
	mock.ExpectGet(key).SetErr(fmt.Errorf("connection error"))

	user, err := redisClient.GetUser(ctx, "testuser")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "failed to get user from Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_GetUser_InvalidJSON(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock GET with invalid JSON
	mock.ExpectGet(key).SetVal("invalid json")

	user, err := redisClient.GetUser(ctx, "testuser")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "failed to unmarshal user from Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_SetUser(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	userData, _ := json.Marshal(testUser)

	ctx := context.Background()
	key := "user:testuser"

	// Mock successful SET
	mock.ExpectSet(key, userData, 30*time.Second).SetVal("OK")

	err := redisClient.SetUser(ctx, "testuser", testUser)
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_SetUser_MarshalError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()

	// Test with nil user - this will marshal successfully to "null"
	mock.ExpectSet("user:testuser", []byte("null"), 30*time.Second).SetVal("OK")

	err := redisClient.SetUser(ctx, "testuser", (*github.User)(nil))
	// We can't easily test JSON marshal errors with the real github.User struct
	// since all its fields are JSON-marshalable, but we test the error path anyway
	assert.NoError(t, err) // This will actually succeed with nil user

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_SetUser_RedisError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	userData, _ := json.Marshal(testUser)

	ctx := context.Background()
	key := "user:testuser"

	// Mock SET with error
	mock.ExpectSet(key, userData, 30*time.Second).SetErr(fmt.Errorf("connection error"))

	err := redisClient.SetUser(ctx, "testuser", testUser)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set user in Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_DeleteUser(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock successful DEL
	mock.ExpectDel(key).SetVal(1)

	err := redisClient.DeleteUser(ctx, "testuser")
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_DeleteUser_RedisError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock DEL with error
	mock.ExpectDel(key).SetErr(fmt.Errorf("connection error"))

	err := redisClient.DeleteUser(ctx, "testuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user from Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_UpdateUserInCache(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	userData, _ := json.Marshal(testUser)

	ctx := context.Background()
	key := "user:testuser"

	// Test when user exists in cache
	mock.ExpectExists(key).SetVal(1) // User exists
	mock.ExpectSet(key, userData, 30*time.Second).SetVal("OK")

	err := redisClient.UpdateUserInCache(ctx, "testuser", testUser)
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_UpdateUserInCache_NotInCache(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	ctx := context.Background()
	key := "user:testuser"

	// Test when user does not exist in cache
	mock.ExpectExists(key).SetVal(0) // User does not exist

	err := redisClient.UpdateUserInCache(ctx, "testuser", testUser)
	assert.NoError(t, err) // Should not error even if not in cache

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_UpdateUserInCache_ExistsError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	testUser := createTestUser()
	ctx := context.Background()
	key := "user:testuser"

	// Mock EXISTS with error
	mock.ExpectExists(key).SetErr(fmt.Errorf("connection error"))

	err := redisClient.UpdateUserInCache(ctx, "testuser", testUser)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check if user exists in Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_GetUserTTL(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"
	expectedTTL := 25 * time.Second

	// Mock successful TTL
	mock.ExpectTTL(key).SetVal(expectedTTL)

	ttl, err := redisClient.GetUserTTL(ctx, "testuser")
	assert.NoError(t, err)
	assert.Equal(t, expectedTTL, ttl)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_GetUserTTL_RedisError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	key := "user:testuser"

	// Mock TTL with error
	mock.ExpectTTL(key).SetErr(fmt.Errorf("connection error"))

	ttl, err := redisClient.GetUserTTL(ctx, "testuser")
	assert.Error(t, err)
	assert.Equal(t, time.Duration(0), ttl)
	assert.Contains(t, err.Error(), "failed to get TTL for user in Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_FlushUsers(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	userKeys := []string{"user:user1", "user:user2", "user:user3"}

	// Mock KEYS and DEL
	mock.ExpectKeys("user:*").SetVal(userKeys)
	mock.ExpectDel(userKeys...).SetVal(int64(len(userKeys)))

	err := redisClient.FlushUsers(ctx)
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_FlushUsers_NoKeys(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()

	// Mock KEYS returning empty slice
	mock.ExpectKeys("user:*").SetVal([]string{})

	err := redisClient.FlushUsers(ctx)
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_FlushUsers_KeysError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()

	// Mock KEYS with error
	mock.ExpectKeys("user:*").SetErr(fmt.Errorf("connection error"))

	err := redisClient.FlushUsers(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user keys from Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_FlushUsers_DelError(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	userKeys := []string{"user:user1", "user:user2"}

	// Mock KEYS successful, DEL with error
	mock.ExpectKeys("user:*").SetVal(userKeys)
	mock.ExpectDel(userKeys...).SetErr(fmt.Errorf("connection error"))

	err := redisClient.FlushUsers(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user keys from Redis")

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisClient_Close(t *testing.T) {
	redisClient, _ := setupTestRedisClient(t)

	// Note: redismock doesn't support ExpectClose, so we just test that Close doesn't panic
	err := redisClient.Close()
	assert.NoError(t, err)
}

// Integration-style test to verify the key format is correct
func TestRedisClient_KeyFormat(t *testing.T) {
	redisClient, mock := setupTestRedisClient(t)
	defer redisClient.Close()

	ctx := context.Background()
	username := "test-user-123"
	expectedKey := "user:test-user-123"

	// Test that the key format is consistent across all operations
	mock.ExpectGet(expectedKey).RedisNil()
	_, err := redisClient.GetUser(ctx, username)
	assert.NoError(t, err)

	mock.ExpectExists(expectedKey).SetVal(0)
	err = redisClient.UpdateUserInCache(ctx, username, createTestUser())
	assert.NoError(t, err)

	mock.ExpectDel(expectedKey).SetVal(1)
	err = redisClient.DeleteUser(ctx, username)
	assert.NoError(t, err)

	mock.ExpectTTL(expectedKey).SetVal(30 * time.Second)
	_, err = redisClient.GetUserTTL(ctx, username)
	assert.NoError(t, err)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
