package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := &config.GitHubConfig{
		BaseURL:    "https://api.github.com",
		APIVersion: "2022-11-28",
		Token:      "test-token",
	}

	client := NewClient(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.config)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestClient_FetchUsers(t *testing.T) {
	tests := []struct {
		name           string
		perPage        int
		since          int
		mockResponse   []User
		mockStatusCode int
		mockError      *APIError
		expectedError  string
		expectedCount  int
	}{
		{
			name:    "successful fetch with valid parameters",
			perPage: 10,
			since:   5,
			mockResponse: []User{
				{ID: 6, Login: "user6", Type: "User"},
				{ID: 7, Login: "user7", Type: "User"},
			},
			mockStatusCode: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:    "successful fetch with zero perPage (should default to maxUsersPerPage)",
			perPage: 0,
			since:   0,
			mockResponse: []User{
				{ID: 1, Login: "user1", Type: "User"},
			},
			mockStatusCode: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:    "successful fetch with excessive perPage (should cap to maxUsersPerPage)",
			perPage: 100,
			since:   0,
			mockResponse: []User{
				{ID: 1, Login: "user1", Type: "User"},
			},
			mockStatusCode: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "API error response",
			perPage:        10,
			since:          0,
			mockStatusCode: http.StatusForbidden,
			mockError: &APIError{
				Message:          "API rate limit exceeded",
				DocumentationURL: "https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting",
			},
			expectedError: "GitHub API error (403): API rate limit exceeded",
		},
		{
			name:           "non-JSON error response",
			perPage:        10,
			since:          0,
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  "GitHub API error (500): Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
				assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
				assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				// Verify query parameters
				assert.Equal(t, "/users", r.URL.Path)
				if tt.perPage > 0 && tt.perPage <= maxUsersPerPage {
					assert.Equal(t, "10", r.URL.Query().Get("per_page"))
				} else {
					assert.Equal(t, "30", r.URL.Query().Get("per_page")) // maxUsersPerPage
				}
				if tt.since > 0 {
					assert.Equal(t, "5", r.URL.Query().Get("since"))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				} else if tt.mockError != nil {
					json.NewEncoder(w).Encode(tt.mockError)
				} else {
					w.Write([]byte("Internal Server Error"))
				}
			}))
			defer server.Close()

			// Create client with mock server URL
			cfg := &config.GitHubConfig{
				BaseURL:    server.URL,
				APIVersion: "2022-11-28",
				Token:      "test-token",
			}
			client := NewClient(cfg)

			// Execute test
			users, err := client.FetchUsers(tt.perPage, tt.since)

			// Assert results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, users)
			} else {
				assert.NoError(t, err)
				assert.Len(t, users, tt.expectedCount)
				if tt.expectedCount > 0 {
					assert.Equal(t, tt.mockResponse, users)
				}
			}
		})
	}
}

func TestClient_FetchAllUsers(t *testing.T) {
	tests := []struct {
		name          string
		totalCount    int
		mockResponses [][]User
		expectedCount int
		expectedError string
	}{
		{
			name:       "fetch exact count",
			totalCount: 45,
			mockResponses: [][]User{
				generateUsers(1, 30),  // First page: 30 users
				generateUsers(31, 15), // Second page: 15 users
			},
			expectedCount: 45,
		},
		{
			name:       "fetch with trimming",
			totalCount: 25,
			mockResponses: [][]User{
				generateUsers(1, 30), // First page: 30 users (will be trimmed to 25)
			},
			expectedCount: 25,
		},
		{
			name:       "fetch with empty response",
			totalCount: 10,
			mockResponses: [][]User{
				{}, // Empty response
			},
			expectedCount: 0,
		},
		{
			name:       "fetch small count",
			totalCount: 5,
			mockResponses: [][]User{
				generateUsers(1, 5),
			},
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseIndex := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				if responseIndex < len(tt.mockResponses) {
					json.NewEncoder(w).Encode(tt.mockResponses[responseIndex])
					responseIndex++
				} else {
					// Return empty array for subsequent requests
					json.NewEncoder(w).Encode([]User{})
				}
			}))
			defer server.Close()

			cfg := &config.GitHubConfig{
				BaseURL:    server.URL,
				APIVersion: "2022-11-28",
				Token:      "test-token",
			}
			client := NewClient(cfg)

			users, err := client.FetchAllUsers(tt.totalCount)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, users, tt.expectedCount)
			}
		})
	}
}

func TestClient_GetUserByUsername(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		mockUser       *User
		mockStatusCode int
		mockError      *APIError
		expectedError  string
	}{
		{
			name:     "successful user fetch",
			username: "testuser",
			mockUser: &User{
				ID:        123,
				Login:     "testuser",
				Type:      "User",
				AvatarURL: "https://github.com/images/error/testuser_happy.gif",
				URL:       "https://api.github.com/users/testuser",
			},
			mockStatusCode: http.StatusOK,
		},
		{
			name:           "user not found",
			username:       "nonexistentuser",
			mockStatusCode: http.StatusNotFound,
			expectedError:  "user not found",
		},
		{
			name:           "API error",
			username:       "testuser",
			mockStatusCode: http.StatusForbidden,
			mockError: &APIError{
				Message:          "API rate limit exceeded",
				DocumentationURL: "https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting",
			},
			expectedError: "GitHub API error (403): API rate limit exceeded",
		},
		{
			name:           "internal server error",
			username:       "testuser",
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  "GitHub API error (500): Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "/users/"+tt.username, r.URL.Path)
				assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
				assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
				assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockStatusCode == http.StatusOK && tt.mockUser != nil {
					json.NewEncoder(w).Encode(tt.mockUser)
				} else if tt.mockError != nil {
					json.NewEncoder(w).Encode(tt.mockError)
				} else if tt.mockStatusCode != http.StatusNotFound {
					w.Write([]byte("Internal Server Error"))
				}
			}))
			defer server.Close()

			cfg := &config.GitHubConfig{
				BaseURL:    server.URL,
				APIVersion: "2022-11-28",
				Token:      "test-token",
			}
			client := NewClient(cfg)

			user, err := client.GetUserByUsername(tt.username)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, tt.mockUser.ID, user.ID)
				assert.Equal(t, tt.mockUser.Login, user.Login)
				assert.Equal(t, tt.mockUser.Type, user.Type)
				assert.Equal(t, tt.mockUser.AvatarURL, user.AvatarURL)
				assert.Equal(t, tt.mockUser.URL, user.URL)
			}
		})
	}
}

func TestClient_NetworkError(t *testing.T) {
	// Test network error handling by using a server that immediately closes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close connection immediately to simulate network error
		panic("connection closed")
	}))
	server.Close() // Close the server to simulate network failure

	cfg := &config.GitHubConfig{
		BaseURL:    server.URL,
		APIVersion: "2022-11-28",
		Token:      "test-token",
	}
	client := NewClient(cfg)

	users, err := client.FetchUsers(10, 0)
	assert.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_InvalidJSON(t *testing.T) {
	// Test invalid JSON response handling
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := &config.GitHubConfig{
		BaseURL:    server.URL,
		APIVersion: "2022-11-28",
		Token:      "test-token",
	}
	client := NewClient(cfg)

	users, err := client.FetchUsers(10, 0)
	assert.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "failed to parse JSON response")
}

// Helper function to generate test users
func generateUsers(startID, count int) []User {
	users := make([]User, count)
	for i := 0; i < count; i++ {
		users[i] = User{
			ID:    startID + i,
			Login: fmt.Sprintf("user%d", startID+i),
			Type:  "User",
		}
	}
	return users
}
