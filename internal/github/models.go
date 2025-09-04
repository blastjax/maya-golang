package github

import "time"

// User represents a GitHub user from the API response
type User struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Login     string `json:"login" gorm:"type:varchar(255);not null;uniqueIndex"`
	AvatarURL string `json:"avatar_url" gorm:"type:text"`
	URL       string `json:"url" gorm:"type:text"`
	Type      string `json:"type" gorm:"type:varchar(50);not null;index"`
	// Additional fields for detailed user info
	Name      *string    `json:"name,omitempty" gorm:"type:varchar(255)"`
	Company   *string    `json:"company,omitempty" gorm:"type:varchar(255)"`
	Blog      *string    `json:"blog,omitempty" gorm:"type:text"`
	Location  *string    `json:"location,omitempty" gorm:"type:varchar(255)"`
	Email     *string    `json:"email,omitempty" gorm:"type:varchar(255)"`
	Bio       *string    `json:"bio,omitempty" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at,omitempty" gorm:"type:timestamp"`
	UpdatedAt time.Time `json:"updated_at,omitempty" gorm:"type:timestamp"`
	SyncedAt  time.Time  `json:"synced_at,omitempty" gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index"`
}

// APIError represents an error response from the GitHub API
type APIError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
}
