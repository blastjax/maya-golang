package api

import (
	"net/http"
	"strconv"

	"github.com/blastjax/maya-golang/internal/github"

	"github.com/gin-gonic/gin"
)

// Handlers contains all the dependencies needed for API handlers
type Handlers struct {
	userRepo     UserRepository
	redisClient  RedisClient
	githubClient GitHubClient
}

// NewHandlers creates a new Handlers instance
func NewHandlers(userRepo UserRepository, redisClient RedisClient, githubClient GitHubClient) *Handlers {
	return &Handlers{
		userRepo:     userRepo,
		redisClient:  redisClient,
		githubClient: githubClient,
	}
}

// UserCreateRequest represents the payload for creating a new user
type UserCreateRequest struct {
	ID        int     `json:"id" binding:"required"`
	Login     string  `json:"login" binding:"required"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	URL       string  `json:"url,omitempty"`
	Type      string  `json:"type" binding:"required"`
	Name      *string `json:"name,omitempty"`
	Company   *string `json:"company,omitempty"`
	Blog      *string `json:"blog,omitempty"`
	Location  *string `json:"location,omitempty"`
	Email     *string `json:"email,omitempty"`
	Bio       *string `json:"bio,omitempty"`
}

// UserUpdateRequest represents the JSON payload for updating user details
type UserUpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	Blog     *string `json:"blog,omitempty"`
	Location *string `json:"location,omitempty"`
	Company  *string `json:"company,omitempty"`
	Bio      *string `json:"bio,omitempty"`
}

// ListUsers handles GET /users - List all users from MySQL
func (h *Handlers) ListUsers(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "30")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid limit parameter. Must be between 1 and 1000",
		})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid offset parameter. Must be >= 0",
		})
		return
	}

	// Get users from database
	users, err := h.userRepo.GetUsers(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve users from database",
		})
		return
	}

	// Get total count for pagination info
	total, err := h.userRepo.GetTotalUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get total user count",
		})
		return
	}

	// Return paginated response
	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"count":  len(users),
	})
}

// GetUser handles GET /users/:username - Get specific user with Redis caching
func (h *Handlers) GetUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username parameter is required",
		})
		return
	}

	ctx := c.Request.Context()

	// Step 1: Check Redis cache first
	cachedUser, err := h.redisClient.GetUser(ctx, username)
	if err != nil {
		// Log error but continue to GitHub fallback
		// In production, you might want to use a proper logger
		c.Header("X-Cache-Error", "true")
	}

	if cachedUser != nil {
		// Cache hit - return cached data
		c.Header("X-Cache-Status", "hit")
		c.JSON(http.StatusOK, gin.H{
			"user":   cachedUser,
			"source": "cache",
		})
		return
	}

	// Step 2: Cache miss - fetch from GitHub
	c.Header("X-Cache-Status", "miss")

	// Fetch user from GitHub API
	githubUser, err := h.fetchUserFromGitHub(username)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch user from GitHub",
		})
		return
	}

	// Step 3: Store in Redis cache (TTL: 30s by default)
	if err := h.redisClient.SetUser(ctx, username, githubUser); err != nil {
		// Log error but don't fail the request
		c.Header("X-Cache-Store-Error", "true")
	}

	// Return user data
	c.JSON(http.StatusOK, gin.H{
		"user":   githubUser,
		"source": "github",
	})
}

// UpdateUser handles PUT /users/:username - Update user details
func (h *Handlers) UpdateUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username parameter is required",
		})
		return
	}

	// Parse JSON payload
	var updateReq UserUpdateRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON payload",
		})
		return
	}

	ctx := c.Request.Context()

	// Step 1: Find user in database by username
	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found in database",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve user from database",
		})
		return
	}

	// Step 2: Update user fields
	updated := false
	if updateReq.Name != nil {
		user.Name = updateReq.Name
		updated = true
	}
	if updateReq.Blog != nil {
		user.Blog = updateReq.Blog
		updated = true
	}
	if updateReq.Location != nil {
		user.Location = updateReq.Location
		updated = true
	}
	if updateReq.Company != nil {
		user.Company = updateReq.Company
		updated = true
	}
	if updateReq.Bio != nil {
		user.Bio = updateReq.Bio
		updated = true
	}

	if !updated {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No valid fields provided for update",
		})
		return
	}

	// Step 3: Update in MySQL database
	if err := h.userRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update user in database",
		})
		return
	}

	// Step 4: Update in Redis cache if cached
	if err := h.redisClient.UpdateUserInCache(ctx, username, user); err != nil {
		// Log error but don't fail the request
		c.Header("X-Cache-Update-Error", "true")
	}

	// Return updated user
	c.JSON(http.StatusOK, gin.H{
		"user":    user,
		"message": "User updated successfully",
	})
}

// fetchUserFromGitHub fetches user data from GitHub API
func (h *Handlers) fetchUserFromGitHub(username string) (*github.User, error) {
	return h.githubClient.GetUserByUsername(username)
}

// DeleteUser handles DELETE /users/:username - Delete user from database and cache
func (h *Handlers) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username parameter is required",
		})
		return
	}

	ctx := c.Request.Context()

	// Step 1: Check if user exists in database
	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found in database",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve user from database",
		})
		return
	}

	// Step 2: Delete from MySQL database
	if err := h.userRepo.DeleteUser(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete user from database",
		})
		return
	}

	// Step 3: Delete from Redis cache (if cached)
	if err := h.redisClient.DeleteUser(ctx, username); err != nil {
		// Log error but don't fail the request since database deletion succeeded
		c.Header("X-Cache-Delete-Error", "true")
	}

	// Return success response with deleted user info
	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
		"user": gin.H{
			"id":    user.ID,
			"login": user.Login,
		},
	})
}

// HealthCheck handles GET /health - Health check endpoint
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "github-users-api",
	})
}

// CreateUser handles POST /user - Create user and store to database
func (h *Handlers) CreateUser(c *gin.Context) {
	// Parse JSON payload
	var newUser UserCreateRequest
	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON payload",
		})
		return
	}

	// Basic validation
	if newUser.Login == "" || newUser.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Both 'id' and 'login' are required",
		})
		return
	}

	// Step 1: Check if user already exists in DB
	existingUser, _ := h.userRepo.GetUserByUsername(newUser.Login)
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "User already exists",
		})
		return
	}

	// Step 2: Build new user struct
	user := &github.User{
		ID:        newUser.ID,
		Login:     newUser.Login,
		AvatarURL: newUser.AvatarURL,
		URL:       newUser.URL,
		Type:      newUser.Type,
		Name:      newUser.Name,
		Company:   newUser.Company,
		Blog:      newUser.Blog,
		Location:  newUser.Location,
		Email:     newUser.Email,
		Bio:       newUser.Bio,
	}

	// Step 3: Insert into MySQL database
	if err := h.userRepo.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to insert user into database",
		})
		return
	}

	// Step 4: Save to Redis cache (optional)
	ctx := c.Request.Context()
	if err := h.redisClient.UpdateUserInCache(ctx, user.Login, user); err != nil {
		c.Header("X-Cache-Insert-Error", "true")
	}

	// Return created user
	c.JSON(http.StatusCreated, gin.H{
		"user":    user,
		"message": "User created successfully",
	})
}
