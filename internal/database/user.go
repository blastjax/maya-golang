package database

import (
	"database/sql"
	"fmt"

	"github.com/blastjax/maya-golang/internal/github"
)

// GetUserByUsername retrieves a user by username from the database
func (r *UserRepository) GetUserByUsername(username string) (*github.User, error) {
	query := `
		SELECT id, login, avatar_url, url, type, name, company, blog, location, email, bio, created_at, updated_at
		FROM users
		WHERE login = ?
	`

	user := &github.User{}
	row := r.db.QueryRow(query, username)

	err := row.Scan(
		&user.ID, &user.Login, &user.AvatarURL, &user.URL, &user.Type,
		&user.Name, &user.Company, &user.Blog, &user.Location,
		&user.Email, &user.Bio, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}

// UpdateUser updates an existing user in the database
func (r *UserRepository) UpdateUser(user *github.User) error {
	query := `
		UPDATE users SET
			avatar_url = ?, url = ?, type = ?, name = ?, company = ?, location = ?, email = ?, bio = ?, created_at = ?, updated_at = ?, synced_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.Exec(query,
		user.AvatarURL, user.URL, user.Type,
		user.Name, user.Company, user.Blog, user.Location, user.Email,
		user.Bio, user.CreatedAt, user.UpdatedAt, user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user %s: %w", user.Login, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user from the database by username
func (r *UserRepository) DeleteUser(username string) error {
	query := `DELETE FROM users WHERE login = ?`

	result, err := r.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user %s: %w", username, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUserByID deletes a user from the database by user ID
func (r *UserRepository) DeleteUserByID(userID int) error {
	query := `DELETE FROM users WHERE id = ?`

	result, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user with ID %d: %w", userID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
