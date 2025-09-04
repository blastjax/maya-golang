package github

import "time"

// User represents a GitHub user from the API response
type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
	URL string `json:"url"`
	Type string `json:"type"`
	// Additional fields for detailed user info
	Name     *string `json:"name,omitempty"`
	Company  *string `json:"company,omitempty"`
	Blog     *string `json:"blog,omitempty"`
	Location *string `json:"location,omitempty"`
	Email    *string `json:"email,omitempty"`
	Bio      *string `json:"bio,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// APIError represents an error response from the GitHub API
type APIError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
}
