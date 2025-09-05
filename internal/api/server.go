package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blastjax/maya-golang/internal/cache"
	"github.com/blastjax/maya-golang/internal/config"
	"github.com/blastjax/maya-golang/internal/database"
	"github.com/blastjax/maya-golang/internal/github"

	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	config      *config.Config
	db          *database.DB
	redisClient *cache.RedisClient
	router      *gin.Engine
	handlers    *Handlers
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize database connection
	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize Redis client
	redisClient, err := cache.NewRedisClient(&cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize GitHub client
	githubClient := github.NewClient(&cfg.GitHub)

	// Initialize user repository
	userRepo := database.NewUserRepository(db)

	// Initialize handlers
	handlers := NewHandlers(userRepo, redisClient, githubClient)

	// Set Gin mode based on environment
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestIDMiddleware())

	server := &Server{
		config:      cfg,
		db:          db,
		redisClient: redisClient,
		router:      router,
		handlers:    handlers,
	}

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// setupRoutes configures all the API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.handlers.HealthCheck)
	s.router.GET("/users", s.handlers.ListUsers)               // GET /users
	s.router.GET("/users/:username", s.handlers.GetUser)       // GET /users/:username
	s.router.PUT("/users/:username", s.handlers.UpdateUser)    // PUT /users/:username
	s.router.DELETE("/users/:username", s.handlers.DeleteUser) // DELETE /users/:username
	s.router.POST("/users", s.handlers.CreateUser)             // POST /users
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port)

	server := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting API server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down API server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("API server stopped")
	return nil
}

// Close closes all connections
func (s *Server) Close() error {
	var lastErr error

	if s.redisClient != nil {
		if err := s.redisClient.Close(); err != nil {
			lastErr = err
			log.Printf("Error closing Redis connection: %v", err)
		}
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			lastErr = err
			log.Printf("Error closing database connection: %v", err)
		}
	}

	return lastErr
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}

// requestIDMiddleware adds a request ID to each request
func requestIDMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Header("X-Request-ID", requestID)
		c.Set("RequestID", requestID)
		c.Next()
	})
}
