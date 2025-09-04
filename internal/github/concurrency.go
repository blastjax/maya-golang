package github

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
)

// PageRequest represents a request to fetch a page of users
type PageRequest struct {
	PerPage int
	Since   int
	Page    int // For tracking which page this is
}

// PageResponse represents the response from fetching a page of users
type PageResponse struct {
	Users []User
	Page  int
	Error error
}

// ConcurrentClient handles concurrent fetching of GitHub users
type ConcurrentClient struct {
	client      *Client
	config      *config.GitHubConfig
	rateLimiter *RateLimiter
}

// RateLimiter implements rate limiting with configurable delay
type RateLimiter struct {
	delay    time.Duration
	lastCall time.Time
	mutex    sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(delay time.Duration) *RateLimiter {
	return &RateLimiter{
		delay: delay,
	}
}

// Wait waits for the appropriate delay before allowing the next call
func (rl *RateLimiter) Wait() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	elapsed := time.Since(rl.lastCall)
	if elapsed < rl.delay {
		time.Sleep(rl.delay - elapsed)
	}
	rl.lastCall = time.Now()
}

// NewConcurrentClient creates a new concurrent GitHub client
func NewConcurrentClient(cfg *config.GitHubConfig) *ConcurrentClient {
	client := NewClient(cfg)
	rateLimiter := NewRateLimiter(cfg.RateLimitDelay)

	return &ConcurrentClient{
		client:      client,
		config:      cfg,
		rateLimiter: rateLimiter,
	}
}

// FetchUsersConcurrently fetches users using a worker pool pattern
func (cc *ConcurrentClient) FetchUsersConcurrently(ctx context.Context, totalCount int, startingSince int) ([]User, error) {
	// Calculate how many pages we need
	pagesNeeded := (totalCount + maxUsersPerPage - 1) / maxUsersPerPage

	// Create channels for work distribution and results
	pageRequests := make(chan PageRequest, pagesNeeded)
	pageResponses := make(chan PageResponse, pagesNeeded)

	// Start worker pool
	var wg sync.WaitGroup
	workerCount := cc.config.MaxWorkers
	if workerCount > pagesNeeded {
		workerCount = pagesNeeded // Don't create more workers than pages
	}

	log.Printf("Starting %d workers to fetch %d pages (total users: %d)",
		workerCount, pagesNeeded, totalCount)

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go cc.worker(ctx, &wg, pageRequests, pageResponses)
	}

	// Send page requests
	go func() {
		defer close(pageRequests)

		currentSince := startingSince
		remainingUsers := totalCount

		for page := 0; page < pagesNeeded; page++ {
			perPage := remainingUsers
			if perPage > maxUsersPerPage {
				perPage = maxUsersPerPage
			}

			select {
			case pageRequests <- PageRequest{
				PerPage: perPage,
				Since:   currentSince,
				Page:    page,
			}:
				// For the next page, we'll need to update 'since' after we get the response
				// But for now, we'll estimate based on the maximum possible user IDs
				currentSince += maxUsersPerPage
				remainingUsers -= perPage
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(pageResponses)
	}()

	// Collect results
	pageResults := make(map[int]PageResponse)
	var errors []error

	for response := range pageResponses {
		if response.Error != nil {
			errors = append(errors, fmt.Errorf("page %d error: %w", response.Page, response.Error))
			continue
		}
		pageResults[response.Page] = response
	}

	// Check for errors
	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to fetch some pages: %v", errors)
	}

	// Combine results in order
	var allUsers []User
	for page := 0; page < pagesNeeded; page++ {
		if result, exists := pageResults[page]; exists {
			allUsers = append(allUsers, result.Users...)
		}
	}

	// Trim to exact count if we got more than requested
	if len(allUsers) > totalCount {
		allUsers = allUsers[:totalCount]
	}

	log.Printf("Successfully fetched %d users across %d pages", len(allUsers), len(pageResults))
	return allUsers, nil
}

// worker processes page requests with rate limiting and retries
func (cc *ConcurrentClient) worker(ctx context.Context, wg *sync.WaitGroup, requests <-chan PageRequest, responses chan<- PageResponse) {
	defer wg.Done()

	for {
		select {
		case req, ok := <-requests:
			if !ok {
				return // Channel closed, worker should exit
			}

			// Process the request with retries
			response := cc.processPageRequestWithRetries(ctx, req)

			select {
			case responses <- response:
				// Response sent successfully
			case <-ctx.Done():
				return // Context cancelled
			}

		case <-ctx.Done():
			return // Context cancelled
		}
	}
}

// processPageRequestWithRetries processes a single page request with retry logic
func (cc *ConcurrentClient) processPageRequestWithRetries(ctx context.Context, req PageRequest) PageResponse {
	var lastErr error

	for attempt := 0; attempt <= cc.config.MaxRetries; attempt++ {
		// Apply rate limiting
		cc.rateLimiter.Wait()

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return PageResponse{
				Page:  req.Page,
				Error: ctx.Err(),
			}
		default:
		}

		// Attempt to fetch the page
		users, err := cc.client.FetchUsers(req.PerPage, req.Since)
		if err == nil {
			log.Printf("Successfully fetched page %d: %d users (since: %d)",
				req.Page, len(users), req.Since)
			return PageResponse{
				Users: users,
				Page:  req.Page,
				Error: nil,
			}
		}

		lastErr = err

		// If this isn't the last attempt, apply backoff
		if attempt < cc.config.MaxRetries {
			backoffDuration := cc.config.RetryBackoffBase * time.Duration(1<<attempt) // Exponential backoff
			log.Printf("Page %d attempt %d failed: %v. Retrying in %v",
				req.Page, attempt+1, err, backoffDuration)

			select {
			case <-time.After(backoffDuration):
				// Continue to retry
			case <-ctx.Done():
				return PageResponse{
					Page:  req.Page,
					Error: ctx.Err(),
				}
			}
		}
	}

	log.Printf("Page %d failed after %d attempts: %v", req.Page, cc.config.MaxRetries+1, lastErr)
	return PageResponse{
		Page:  req.Page,
		Error: fmt.Errorf("failed after %d attempts: %w", cc.config.MaxRetries+1, lastErr),
	}
}

// FetchUsersSequentially provides a fallback sequential method
func (cc *ConcurrentClient) FetchUsersSequentially(totalCount int, startingSince int) ([]User, error) {
	ctx := context.Background()
	var allUsers []User
	since := startingSince

	for len(allUsers) < totalCount {
		remaining := totalCount - len(allUsers)
		perPage := remaining
		if perPage > maxUsersPerPage {
			perPage = maxUsersPerPage
		}

		// Apply rate limiting
		cc.rateLimiter.Wait()

		users, err := cc.client.FetchUsers(perPage, since)
		if err != nil {
			return allUsers, fmt.Errorf("failed to fetch users at since=%d: %w", since, err)
		}

		if len(users) == 0 {
			// No more users available
			break
		}

		allUsers = append(allUsers, users...)

		// Update since to the last user ID for next page
		if len(users) > 0 {
			since = users[len(users)-1].ID
		}

		// If we got fewer users than requested, we've reached the end
		if len(users) < perPage {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return allUsers, ctx.Err()
		default:
		}
	}

	// Trim to exact count if we got more than requested
	if len(allUsers) > totalCount {
		allUsers = allUsers[:totalCount]
	}

	return allUsers, nil
}
