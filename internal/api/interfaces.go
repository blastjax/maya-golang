package api

import (
	"context"

	"github.com/blastjax/maya-golang/internal/github"
)

// UserRepository interface defines the methods for user database operations
type UserRepository interface {
	GetUsers(limit, offset int) ([]github.User, error)
	GetTotalUsers() (int, error)
	GetUserByUsername(username string) (*github.User, error)
	UpdateUser(user *github.User) error
	DeleteUser(username string) error
	InsertUser(user *github.User) error
	InsertUsers(users []github.User) error
	GetUser(id int) (*github.User, error)
	GetLastSyncedUserID() (int, error)
}

// RedisClient interface defines the methods for Redis cache operations
type RedisClient interface {
	GetUser(ctx context.Context, username string) (*github.User, error)
	SetUser(ctx context.Context, username string, user *github.User) error
	DeleteUser(ctx context.Context, username string) error
	UpdateUserInCache(ctx context.Context, username string, user *github.User) error
	Close() error
}

// GitHubClient interface defines the methods for GitHub API operations
type GitHubClient interface {
	GetUserByUsername(username string) (*github.User, error)
	FetchUsers(perPage, since int) ([]github.User, error)
}
