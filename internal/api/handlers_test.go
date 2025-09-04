package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blastjax/maya-golang/internal/github"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUsers(limit, offset int) ([]github.User, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]github.User), args.Error(1)
}

func (m *MockUserRepository) GetTotalUsers() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *MockUserRepository) GetUserByUsername(username string) (*github.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*github.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(user *github.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) DeleteUser(username string) error {
	args := m.Called(username)
	return args.Error(0)
}

func (m *MockUserRepository) InsertUser(user *github.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) InsertUsers(users []github.User) error {
	args := m.Called(users)
	return args.Error(0)
}

func (m *MockUserRepository) GetUser(id int) (*github.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*github.User), args.Error(1)
}

func (m *MockUserRepository) GetLastSyncedUserID() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) GetUser(ctx context.Context, username string) (*github.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*github.User), args.Error(1)
}

func (m *MockRedisClient) SetUser(ctx context.Context, username string, user *github.User) error {
	args := m.Called(ctx, username, user)
	return args.Error(0)
}

func (m *MockRedisClient) DeleteUser(ctx context.Context, username string) error {
	args := m.Called(ctx, username)
	return args.Error(0)
}

func (m *MockRedisClient) UpdateUserInCache(ctx context.Context, username string, user *github.User) error {
	args := m.Called(ctx, username, user)
	return args.Error(0)
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockGitHubClient struct {
	mock.Mock
}

func (m *MockGitHubClient) GetUserByUsername(username string) (*github.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*github.User), args.Error(1)
}

func (m *MockGitHubClient) FetchUsers(perPage, since int) ([]github.User, error) {
	args := m.Called(perPage, since)
	return args.Get(0).([]github.User), args.Error(1)
}

// Helper functions
func setupTestHandlers() (*Handlers, *MockUserRepository, *MockRedisClient, *MockGitHubClient) {
	mockUserRepo := &MockUserRepository{}
	mockRedisClient := &MockRedisClient{}
	mockGitHubClient := &MockGitHubClient{}

	handlers := NewHandlers(mockUserRepo, mockRedisClient, mockGitHubClient)

	return handlers, mockUserRepo, mockRedisClient, mockGitHubClient
}

func createTestUser(id int, login string) *github.User {
	name := login + " Name"
	company := "Test Company"
	return &github.User{
		ID:        id,
		Login:     login,
		Type:      "User",
		Name:      &name,
		Company:   &company,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestNewHandlers(t *testing.T) {
	handlers, mockUserRepo, mockRedisClient, mockGitHubClient := setupTestHandlers()

	assert.NotNil(t, handlers)
	assert.Equal(t, mockUserRepo, handlers.userRepo)
	assert.Equal(t, mockRedisClient, handlers.redisClient)
	assert.Equal(t, mockGitHubClient, handlers.githubClient)
}

func TestHandlers_ListUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		mockUsers      []github.User
		mockTotal      int
		mockUserError  error
		mockTotalError error
		expectedStatus int
		expectedCount  int
	}{
		{
			name:        "successful list with default params",
			queryParams: "",
			mockUsers: []github.User{
				*createTestUser(1, "user1"),
				*createTestUser(2, "user2"),
			},
			mockTotal:      2,
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:        "successful list with custom params",
			queryParams: "?limit=5&offset=10",
			mockUsers: []github.User{
				*createTestUser(11, "user11"),
			},
			mockTotal:      50,
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "invalid limit parameter",
			queryParams:    "?limit=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid offset parameter",
			queryParams:    "?offset=-1",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "limit too high",
			queryParams:    "?limit=2000",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "user repository error",
			queryParams:    "",
			mockUserError:  fmt.Errorf("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "total count error",
			queryParams: "",
			mockUsers: []github.User{
				*createTestUser(1, "user1"),
			},
			mockTotalError: fmt.Errorf("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockUserRepo, _, _ := setupTestHandlers()

			// Setup expectations
			if tt.expectedStatus != http.StatusBadRequest {
				limit := 30
				offset := 0
				if strings.Contains(tt.queryParams, "limit=5") {
					limit = 5
				}
				if strings.Contains(tt.queryParams, "offset=10") {
					offset = 10
				}

				mockUserRepo.On("GetUsers", limit, offset).Return(tt.mockUsers, tt.mockUserError)

				if tt.mockUserError == nil {
					mockUserRepo.On("GetTotalUsers").Return(tt.mockTotal, tt.mockTotalError)
				}
			}

			// Create request
			req := httptest.NewRequest("GET", "/users"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Create Gin context
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/users", handlers.ListUsers)
			router.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				users := response["users"].([]interface{})
				assert.Len(t, users, tt.expectedCount)
				assert.Equal(t, float64(tt.mockTotal), response["total"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandlers_GetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                string
		username            string
		mockCachedUser      *github.User
		mockCacheError      error
		mockGitHubUser      *github.User
		mockGitHubError     error
		mockSetCacheError   error
		expectedStatus      int
		expectedSource      string
		expectedCacheStatus string
	}{
		{
			name:                "cache hit",
			username:            "testuser",
			mockCachedUser:      createTestUser(1, "testuser"),
			expectedStatus:      http.StatusOK,
			expectedSource:      "cache",
			expectedCacheStatus: "hit",
		},
		{
			name:                "cache miss, GitHub success",
			username:            "testuser",
			mockCachedUser:      nil,
			mockGitHubUser:      createTestUser(1, "testuser"),
			expectedStatus:      http.StatusOK,
			expectedSource:      "github",
			expectedCacheStatus: "miss",
		},
		{
			name:                "cache miss, user not found",
			username:            "nonexistent",
			mockCachedUser:      nil,
			mockGitHubError:     fmt.Errorf("user not found"),
			expectedStatus:      http.StatusNotFound,
			expectedCacheStatus: "miss",
		},
		{
			name:                "cache miss, GitHub error",
			username:            "testuser",
			mockCachedUser:      nil,
			mockGitHubError:     fmt.Errorf("GitHub API error"),
			expectedStatus:      http.StatusInternalServerError,
			expectedCacheStatus: "miss",
		},
		{
			name:           "missing username parameter",
			username:       "",
			expectedStatus: http.StatusNotFound, // Route doesn't match when username is empty
		},
		{
			name:                "cache error (continue to GitHub)",
			username:            "testuser",
			mockCacheError:      fmt.Errorf("Redis error"),
			mockGitHubUser:      createTestUser(1, "testuser"),
			expectedStatus:      http.StatusOK,
			expectedSource:      "github",
			expectedCacheStatus: "miss",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, _, mockRedisClient, mockGitHubClient := setupTestHandlers()

			// Setup expectations
			if tt.username != "" {
				mockRedisClient.On("GetUser", mock.Anything, tt.username).Return(tt.mockCachedUser, tt.mockCacheError)

				// If cache miss or cache error, expect GitHub call
				if tt.mockCachedUser == nil {
					mockGitHubClient.On("GetUserByUsername", tt.username).Return(tt.mockGitHubUser, tt.mockGitHubError)

					// If GitHub success, expect cache set
					if tt.mockGitHubUser != nil && tt.mockGitHubError == nil {
						mockRedisClient.On("SetUser", mock.Anything, tt.username, tt.mockGitHubUser).Return(tt.mockSetCacheError)
					}
				}
			}

			// Create request
			url := "/users/testuser"
			if tt.username == "" {
				url = "/users/"
			} else if tt.username != "testuser" {
				url = "/users/" + tt.username
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			// Create Gin context
			router := gin.New()
			router.GET("/users/:username", handlers.GetUser)
			router.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCacheStatus != "" {
				assert.Equal(t, tt.expectedCacheStatus, w.Header().Get("X-Cache-Status"))
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, tt.expectedSource, response["source"])
				assert.NotNil(t, response["user"])
			}

			mockRedisClient.AssertExpectations(t)
			mockGitHubClient.AssertExpectations(t)
		})
	}
}

func TestHandlers_UpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                 string
		username             string
		requestBody          UserUpdateRequest
		mockUser             *github.User
		mockGetUserError     error
		mockUpdateUserError  error
		mockUpdateCacheError error
		expectedStatus       int
	}{
		{
			name:     "successful update",
			username: "testuser",
			requestBody: UserUpdateRequest{
				Name:     stringPtr("Updated Name"),
				Location: stringPtr("New Location"),
			},
			mockUser:       createTestUser(1, "testuser"),
			expectedStatus: http.StatusOK,
		},
		{
			name:     "user not found",
			username: "nonexistent",
			requestBody: UserUpdateRequest{
				Name: stringPtr("Updated Name"),
			},
			mockGetUserError: fmt.Errorf("user not found"),
			expectedStatus:   http.StatusNotFound,
		},
		{
			name:           "missing username",
			username:       "",
			expectedStatus: http.StatusNotFound, // Route doesn't match when username is empty
		},
		{
			name:           "invalid JSON",
			username:       "testuser",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:             "no fields to update",
			username:         "testuser",
			requestBody:      UserUpdateRequest{},
			mockUser:         createTestUser(1, "testuser"),
			mockGetUserError: nil, // GetUserByUsername will be called
			expectedStatus:   http.StatusBadRequest,
		},
		{
			name:     "database error on get",
			username: "testuser",
			requestBody: UserUpdateRequest{
				Name: stringPtr("Updated Name"),
			},
			mockGetUserError: fmt.Errorf("database error"),
			expectedStatus:   http.StatusInternalServerError,
		},
		{
			name:     "database error on update",
			username: "testuser",
			requestBody: UserUpdateRequest{
				Name: stringPtr("Updated Name"),
			},
			mockUser:            createTestUser(1, "testuser"),
			mockUpdateUserError: fmt.Errorf("database error"),
			expectedStatus:      http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockUserRepo, mockRedisClient, _ := setupTestHandlers()

			// Setup expectations
			if tt.username != "" && tt.name != "invalid JSON" {
				mockUserRepo.On("GetUserByUsername", tt.username).Return(tt.mockUser, tt.mockGetUserError)

				if tt.mockUser != nil && tt.mockGetUserError == nil && len(getFieldsToUpdate(tt.requestBody)) > 0 {
					mockUserRepo.On("UpdateUser", mock.AnythingOfType("*github.User")).Return(tt.mockUpdateUserError)

					if tt.mockUpdateUserError == nil {
						mockRedisClient.On("UpdateUserInCache", mock.Anything, tt.username, mock.AnythingOfType("*github.User")).Return(tt.mockUpdateCacheError)
					}
				}
			}

			// Create request body
			var body []byte
			if tt.name == "invalid JSON" {
				body = []byte("invalid json")
			} else {
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			// Create request
			url := "/users/testuser"
			if tt.username == "" {
				url = "/users/"
			} else if tt.username != "testuser" {
				url = "/users/" + tt.username
			}

			req := httptest.NewRequest("PUT", url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Create Gin context
			router := gin.New()
			router.PUT("/users/:username", handlers.UpdateUser)
			router.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "User updated successfully", response["message"])
				assert.NotNil(t, response["user"])
			}

			mockUserRepo.AssertExpectations(t)
			mockRedisClient.AssertExpectations(t)
		})
	}
}

func TestHandlers_DeleteUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                 string
		username             string
		mockUser             *github.User
		mockGetUserError     error
		mockDeleteUserError  error
		mockDeleteCacheError error
		expectedStatus       int
	}{
		{
			name:           "successful delete",
			username:       "testuser",
			mockUser:       createTestUser(1, "testuser"),
			expectedStatus: http.StatusOK,
		},
		{
			name:             "user not found",
			username:         "nonexistent",
			mockGetUserError: fmt.Errorf("user not found"),
			expectedStatus:   http.StatusNotFound,
		},
		{
			name:           "missing username",
			username:       "",
			expectedStatus: http.StatusNotFound, // Route doesn't match when username is empty
		},
		{
			name:             "database error on get",
			username:         "testuser",
			mockGetUserError: fmt.Errorf("database error"),
			expectedStatus:   http.StatusInternalServerError,
		},
		{
			name:                "database error on delete",
			username:            "testuser",
			mockUser:            createTestUser(1, "testuser"),
			mockDeleteUserError: fmt.Errorf("database error"),
			expectedStatus:      http.StatusInternalServerError,
		},
		{
			name:                 "cache delete error (should not fail request)",
			username:             "testuser",
			mockUser:             createTestUser(1, "testuser"),
			mockDeleteCacheError: fmt.Errorf("Redis error"),
			expectedStatus:       http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockUserRepo, mockRedisClient, _ := setupTestHandlers()

			// Setup expectations
			if tt.username != "" {
				mockUserRepo.On("GetUserByUsername", tt.username).Return(tt.mockUser, tt.mockGetUserError)

				if tt.mockUser != nil && tt.mockGetUserError == nil {
					mockUserRepo.On("DeleteUser", tt.username).Return(tt.mockDeleteUserError)

					if tt.mockDeleteUserError == nil {
						mockRedisClient.On("DeleteUser", mock.Anything, tt.username).Return(tt.mockDeleteCacheError)
					}
				}
			}

			// Create request
			url := "/users/testuser"
			if tt.username == "" {
				url = "/users/"
			} else if tt.username != "testuser" {
				url = "/users/" + tt.username
			}

			req := httptest.NewRequest("DELETE", url, nil)
			w := httptest.NewRecorder()

			// Create Gin context
			router := gin.New()
			router.DELETE("/users/:username", handlers.DeleteUser)
			router.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "User deleted successfully", response["message"])
				assert.NotNil(t, response["user"])

				user := response["user"].(map[string]interface{})
				assert.Equal(t, float64(tt.mockUser.ID), user["id"])
				assert.Equal(t, tt.mockUser.Login, user["login"])

				// Check for cache delete error header
				if tt.mockDeleteCacheError != nil {
					assert.Equal(t, "true", w.Header().Get("X-Cache-Delete-Error"))
				}
			}

			mockUserRepo.AssertExpectations(t)
			mockRedisClient.AssertExpectations(t)
		})
	}
}

func TestHandlers_HealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers, _, _, _ := setupTestHandlers()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router := gin.New()
	router.GET("/health", handlers.HealthCheck)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "github-users-api", response["service"])
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func getFieldsToUpdate(req UserUpdateRequest) []string {
	fields := []string{}
	if req.Name != nil {
		fields = append(fields, "name")
	}
	if req.Blog != nil {
		fields = append(fields, "blog")
	}
	if req.Location != nil {
		fields = append(fields, "location")
	}
	if req.Company != nil {
		fields = append(fields, "company")
	}
	if req.Bio != nil {
		fields = append(fields, "bio")
	}
	return fields
}
