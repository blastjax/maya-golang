package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/github"
)

// UserRepository handles user database operations
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// InsertUser inserts a new user into the database
func (r *UserRepository) InsertUser(user *github.User) error {
	query := `
		INSERT INTO users (
			id, login, avatar_url, url, type, name, company, blog,
			location, email, bio, created_at, updated_at, synced_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NOW()
		) ON DUPLICATE KEY UPDATE
			login = VALUES(login),
			avatar_url = VALUES(avatar_url),
			url = VALUES(url),
			type = VALUES(type),
			name = VALUES(name),
			company = VALUES(company),
			blog = VALUES(blog),
			location = VALUES(location),
			email = VALUES(email),
			bio = VALUES(bio),
			updated_at = NOW(),
			synced_at = NOW()
	`

	_, err := r.db.Exec(query,
		user.ID, user.Login, user.AvatarURL, user.URL, user.Type,
		user.Name, user.Company, user.Blog, user.Location,
		user.Email, user.Bio,
	)

	if err != nil {
		return fmt.Errorf("failed to insert user %s: %w", user.Login, err)
	}

	return nil
}

// InsertUsers inserts multiple users in a batch
func (r *UserRepository) InsertUsers(users []github.User) error {
	if len(users) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, user := range users {
		if err := r.InsertUser(&user); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetUser retrieves a user by ID
func (r *UserRepository) GetUser(id int) (*github.User, error) {
	query := `
		SELECT id, login, avatar_url, url, type, name, company, blog, location, email, bio, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	user := &github.User{}
	row := r.db.QueryRow(query, id)

	err := row.Scan(
		&user.ID, &user.Login, &user.AvatarURL, &user.URL, &user.Type,
		&user.Name, &user.Company, &user.Blog, &user.Location,
		&user.Email, &user.Bio, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUsers retrieves users with pagination
func (r *UserRepository) GetUsers(limit, offset int) ([]github.User, error) {
	query := `
		SELECT id, login, avatar_url, url, type, name, company, blog, location, email, bio, created_at, updated_at
		FROM users
		ORDER BY id
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []github.User
	for rows.Next() {
		user := github.User{}
		err := rows.Scan(
			&user.ID, &user.Login, &user.AvatarURL, &user.URL, &user.Type,
			&user.Name, &user.Company, &user.Blog, &user.Location,
			&user.Email, &user.Bio, &user.CreatedAt, &user.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

// GetTotalUsers returns the total number of users in the database
func (r *UserRepository) GetTotalUsers() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// GetLastSyncedUserID returns the highest user ID that has been synced
func (r *UserRepository) GetLastSyncedUserID() (int, error) {
	var lastID sql.NullInt64
	err := r.db.QueryRow("SELECT MAX(id) FROM users").Scan(&lastID)
	if err != nil {
		return 0, fmt.Errorf("failed to get last synced user ID: %w", err)
	}

	if !lastID.Valid {
		return 0, nil
	}

	return int(lastID.Int64), nil
}

// GetUsersCreatedAfter returns users created after a specific time
func (r *UserRepository) GetUsersCreatedAfter(after time.Time) ([]github.User, error) {
	query := `
		SELECT id, login, avatar_url, url, type, name, company, blog, location, email, bio, created_at, updated_at
		FROM users
		WHERE synced_at > ?
		ORDER BY id
	`

	rows, err := r.db.Query(query, after)
	if err != nil {
		return nil, fmt.Errorf("failed to query users created after %v: %w", after, err)
	}
	defer rows.Close()

	var users []github.User
	for rows.Next() {
		user := github.User{}
		err := rows.Scan(
			&user.ID, &user.Login, &user.AvatarURL, &user.URL, &user.Type,
			&user.Name, &user.Company, &user.Blog, &user.Location,
			&user.Email, &user.Bio, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}
