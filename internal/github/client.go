package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
)

const (
	// User-Agent header
	userAgent = "maya-golang-cli/1.0"

	// GitHub API specs
	maxUsersPerPage = 30 // GitHub API limit for users endpoint
)

// Client represents a GitHub API client
type Client struct {
	httpClient *http.Client
	config     *config.GitHubConfig
}

// NewClient creates a new GitHub API client
func NewClient(cfg *config.GitHubConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

// FetchUsers fetches users from the GitHub API with proper pagination
func (c *Client) FetchUsers(perPage, since int) ([]User, error) {
	// Validate and adjust perPage according to GitHub API specs
	if perPage <= 0 || perPage > maxUsersPerPage {
		perPage = maxUsersPerPage
	}

	// Construct the URL with query parameters
	u, err := url.Parse(c.config.BaseURL + "/users")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("per_page", strconv.Itoa(perPage))
	if since > 0 {
		q.Set("since", strconv.Itoa(since))
	}
	u.RawQuery = q.Encode()

	// Create the HTTP request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", c.config.APIVersion)
	req.Header.Set("User-Agent", userAgent)

	// Add GitHub token if available
	if c.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Token)
	}

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return users, nil
}

// FetchAllUsers fetches multiple pages of users until reaching the desired count
func (c *Client) FetchAllUsers(totalCount int) ([]User, error) {
	var allUsers []User
	since := 0

	for len(allUsers) < totalCount {
		remaining := totalCount - len(allUsers)
		perPage := remaining
		if perPage > maxUsersPerPage {
			perPage = maxUsersPerPage
		}

		users, err := c.FetchUsers(perPage, since)
		if err != nil {
			return allUsers, err
		}

		if len(users) == 0 {
			// No more users available
			break
		}

		allUsers = append(allUsers, users...)

		// Update since to the last user ID for next page
		if len(users) > 0 {
			since = users[len(users)-1].ID
		}

		// If we got fewer users than requested, we've reached the end
		if len(users) < perPage {
			break
		}
	}

	// Trim to exact count if we got more than requested
	if len(allUsers) > totalCount {
		allUsers = allUsers[:totalCount]
	}

	return allUsers, nil
}
