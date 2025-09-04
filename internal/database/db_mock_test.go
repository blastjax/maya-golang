package database

import (
	"testing"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_GetDSN(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
	}

	expected := "testuser:testpass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	actual := cfg.GetDSN()
	assert.Equal(t, expected, actual)
}

func TestDatabaseConfig_GetDSN_WithEmptyPassword(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "testuser",
		Password: "",
		Name:     "testdb",
	}

	expected := "testuser:@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	actual := cfg.GetDSN()
	assert.Equal(t, expected, actual)
}

func TestNew_InvalidConfig(t *testing.T) {
	// Test with invalid configuration to ensure error handling works
	cfg := &config.DatabaseConfig{
		Host:               "invalid-host",
		Port:               3306,
		User:               "testuser",
		Password:           "testpass",
		Name:               "testdb",
		MaxConnections:     10,
		MaxIdleConnections: 5,
	}

	// This should fail because the host is invalid
	db, err := New(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to open database")
}
