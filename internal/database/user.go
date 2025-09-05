package database

import (
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/github"
	"gorm.io/gorm"
)

// GetUserByUsername retrieves a user by username from the database using GORM
func (r *UserRepository) GetUserByUsername(username string) (*github.User, error) {
	var user github.User
	result := r.db.Where("login = ?", username).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by username: %w", result.Error)
	}

	return &user, nil
}

// UpdateUser updates an existing user in the database using GORM
func (r *UserRepository) UpdateUser(user *github.User) error {
	// Set UpdatedAt to current time
	user.UpdatedAt = time.Now()

	result := r.db.Save(user)

	if result.Error != nil {
		return fmt.Errorf("failed to update user %s: %w", user.Login, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user from the database by username using GORM
func (r *UserRepository) DeleteUser(username string) error {
	result := r.db.Where("login = ?", username).Delete(&github.User{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete user %s: %w", username, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUserByID deletes a user from the database by user ID using GORM
func (r *UserRepository) DeleteUserByID(userID int) error {
	result := r.db.Delete(&github.User{}, userID)

	if result.Error != nil {
		return fmt.Errorf("failed to delete user with ID %d: %w", userID, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// CreateUser inserts a new user into the database using GORM
func (r *UserRepository) CreateUser(user *github.User) error {
	// Set CreatedAt and UpdatedAt to current time
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = time.Now()
	}

	result := r.db.Create(user)
	if result.Error != nil {
		return fmt.Errorf("failed to create user %s: %w", user.Login, result.Error)
	}

	return nil
}
