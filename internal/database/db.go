package database

import (
	"fmt"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/blastjax/maya-golang/internal/github"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB represents a database connection using GORM
type DB struct {
	*gorm.DB
}

// New creates a new database connection using GORM
func New(cfg *config.DatabaseConfig) (*DB, error) {
	// Configure GORM logger level based on environment
	gormLogger := logger.Default.LogMode(logger.Info)

	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConnections)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// CreateSchema creates the necessary database schema using GORM auto-migration
func (db *DB) CreateSchema() error {
	// Auto-migrate the User model
	if err := db.AutoMigrate(&github.User{}); err != nil {
		return fmt.Errorf("failed to auto-migrate schema: %w", err)
	}

	return nil
}
