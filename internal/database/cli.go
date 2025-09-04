package database

import (
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/github"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository handles user database operations
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// InsertUser inserts a new user into the database using GORM upsert
func (r *UserRepository) InsertUser(user *github.User) error {
	// Set SyncedAt to current time
	user.SyncedAt = time.Now()

	// Use GORM's upsert functionality with OnConflict
	result := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"login", "avatar_url", "url", "type", "name", "company",
			"blog", "location", "email", "bio", "created_at", "updated_at", "synced_at",
		}),
	}).Create(user)

	if result.Error != nil {
		return fmt.Errorf("failed to insert user %s: %w", user.Login, result.Error)
	}

	return nil
}

// InsertUsers inserts multiple users in a batch using GORM
func (r *UserRepository) InsertUsers(users []github.User) error {
	if len(users) == 0 {
		return nil
	}

	// Set SyncedAt for all users
	for i := range users {
		users[i].SyncedAt = time.Now()
	}

	// Use GORM transaction for batch insert with upsert
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"login", "avatar_url", "url", "type", "name", "company",
				"blog", "location", "email", "bio", "created_at", "updated_at", "synced_at",
			}),
		}).CreateInBatches(users, 100) // Insert in batches of 100

		if result.Error != nil {
			return fmt.Errorf("failed to batch insert users: %w", result.Error)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to insert users in transaction: %w", err)
	}

	return nil
}

// GetUser retrieves a user by ID using GORM
func (r *UserRepository) GetUser(id int) (*github.User, error) {
	var user github.User
	result := r.db.First(&user, id)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", result.Error)
	}

	return &user, nil
}

// GetUsers retrieves users with pagination using GORM
func (r *UserRepository) GetUsers(limit, offset int) ([]github.User, error) {
	var users []github.User
	result := r.db.Order("id").Limit(limit).Offset(offset).Find(&users)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to query users: %w", result.Error)
	}

	return users, nil
}

// GetTotalUsers returns the total number of users in the database using GORM
func (r *UserRepository) GetTotalUsers() (int, error) {
	var count int64
	result := r.db.Model(&github.User{}).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count users: %w", result.Error)
	}
	return int(count), nil
}

// GetLastSyncedUserID returns the highest user ID that has been synced using GORM
func (r *UserRepository) GetLastSyncedUserID() (int, error) {
	var user github.User
	result := r.db.Order("id desc").First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return 0, nil // No users found
		}
		return 0, fmt.Errorf("failed to get last synced user ID: %w", result.Error)
	}

	return user.ID, nil
}

// GetUsersCreatedAfter returns users created after a specific time using GORM
func (r *UserRepository) GetUsersCreatedAfter(after time.Time) ([]github.User, error) {
	var users []github.User
	result := r.db.Where("synced_at > ?", after).Order("id").Find(&users)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to query users created after %v: %w", after, result.Error)
	}

	return users, nil
}
