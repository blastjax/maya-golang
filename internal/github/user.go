package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetUserByUsername fetches a specific user by username from GitHub API
func (c *Client) GetUserByUsername(username string) (*User, error) {
	// Construct the URL for the specific user
	url := fmt.Sprintf("%s/users/%s", c.config.BaseURL, username)

	// Create the HTTP request
	req, err := http.NewRequest("GET", url, nil)
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

	// Handle different status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Parse the JSON response
		var user User
		if err := json.Unmarshal(body, &user); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
		return &user, nil

	case http.StatusNotFound:
		return nil, fmt.Errorf("user not found")

	default:
		// Handle other error status codes
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}
}
