package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig
	GitHub   GitHubConfig
	Redis    RedisConfig
	API      APIConfig
	App      AppConfig
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Host               string
	Port               int
	User               string
	Password           string
	Name               string
	MaxConnections     int
	MaxIdleConnections int
}

// GitHubConfig holds GitHub API-related configuration
type GitHubConfig struct {
	Token            string
	BaseURL          string
	APIVersion       string
	MaxWorkers       int
	RateLimitDelay   time.Duration
	MaxRetries       int
	RetryBackoffBase time.Duration
}

// RedisConfig holds Redis-related configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	TTL      time.Duration
}

// APIConfig holds API server configuration
type APIConfig struct {
	Host string
	Port int
}

// AppConfig holds general application configuration
type AppConfig struct {
	Environment string
	LogLevel    string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		Database: DatabaseConfig{
			Host:               getEnv("DB_HOST", "localhost"),
			Port:               getEnvAsInt("DB_PORT", 3306),
			User:               getEnv("DB_USER", "root"),
			Password:           getEnv("DB_PASSWORD", ""),
			Name:               getEnv("DB_NAME", "github_users"),
			MaxConnections:     getEnvAsInt("DB_MAX_CONNECTIONS", 10),
			MaxIdleConnections: getEnvAsInt("DB_MAX_IDLE_CONNECTIONS", 5),
		},
		GitHub: GitHubConfig{
			Token:            getEnv("GITHUB_TOKEN", ""),
			BaseURL:          getEnv("GITHUB_API_BASE_URL", "https://api.github.com"),
			APIVersion:       getEnv("GITHUB_API_VERSION", "2022-11-28"),
			MaxWorkers:       getEnvAsInt("GITHUB_MAX_WORKERS", 5),
			RateLimitDelay:   time.Duration(getEnvAsInt("GITHUB_RATE_LIMIT_DELAY_MS", 1000)) * time.Millisecond,
			MaxRetries:       getEnvAsInt("GITHUB_MAX_RETRIES", 3),
			RetryBackoffBase: time.Duration(getEnvAsInt("GITHUB_RETRY_BACKOFF_MS", 500)) * time.Millisecond,
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			TTL:      time.Duration(getEnvAsInt("REDIS_TTL_SECONDS", 30)) * time.Second,
		},
		API: APIConfig{
			Host: getEnv("API_HOST", "localhost"),
			Port: getEnvAsInt("API_PORT", 8080),
		},
		App: AppConfig{
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}
	return nil
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Name)
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsInt gets an environment variable as integer with a fallback value
func getEnvAsInt(name string, fallback int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}
