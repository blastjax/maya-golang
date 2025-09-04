package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

// DB represents a database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	db, err := sql.Open("mysql", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// CreateSchema creates the necessary database schema
func (db *DB) CreateSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		login VARCHAR(255) NOT NULL UNIQUE,
		avatar_url TEXT,
		url TEXT,
		type VARCHAR(50) NOT NULL,
		name VARCHAR(255),
		company VARCHAR(255),
		blog TEXT,
		location VARCHAR(255),
		email VARCHAR(255),
		bio TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP,
		synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_login (login),
		INDEX idx_type (type),
		INDEX idx_synced_at (synced_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}
