package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/blastjax/maya-golang/internal/config"
	"github.com/blastjax/maya-golang/internal/database"
	"github.com/blastjax/maya-golang/internal/github"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "sync-users":
		syncUsersCommand(cfg)
	case "list-users":
		listUsersCommand(cfg)
	case "setup-db":
		setupDBCommand(cfg)
	}

}

func syncUsersCommand(cfg *config.Config) {
	// Create a new FlagSet for the sync-users subcommand
	syncUsersCmd := flag.NewFlagSet("sync-users", flag.ExitOnError)

	var (
		count      = syncUsersCmd.Int("count", 30, "Number of users to fetch (max 1000)")
		since      = syncUsersCmd.Int("since", 0, "User ID to start from (0 = auto-detect)")
		format     = syncUsersCmd.String("format", "table", "Output format: table, json, detailed")
		store      = syncUsersCmd.Bool("store", false, "Store users in database")
		concurrent = syncUsersCmd.Bool("concurrent", true, "Use concurrent fetching (recommended for large counts)")
		help       = syncUsersCmd.Bool("help", false, "Show help message")
	)

	syncUsersCmd.Usage = func() {
		printSyncUsersUsage()
	}

	// Parse the arguments starting from position 2 (after sync-users)
	syncUsersCmd.Parse(os.Args[2:])

	if *help {
		printSyncUsersUsage()
		os.Exit(0)
	}

	// Initialize database if storing users
	var db *database.DB
	var userRepo *database.UserRepository

	if *store {
		var err error
		db, err = database.New(&cfg.Database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		userRepo = database.NewUserRepository(db)

		// Auto-detect since value if not provided and storing
		if *since == 0 {
			lastID, err := userRepo.GetLastSyncedUserID()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting last synced user ID: %v\n", err)
				os.Exit(1)
			}
			*since = lastID
		}
	}

	var users []github.User
	var err error

	startTime := time.Now()

	if *concurrent && *count > 30 {
		// Use concurrent client for larger requests
		concurrentClient := github.NewConcurrentClient(&cfg.GitHub)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		log.Printf("Using concurrent fetching with %d workers for %d users",
			cfg.GitHub.MaxWorkers, *count)

		users, err = concurrentClient.FetchUsersConcurrently(ctx, *count, *since)
	} else {
		// Use sequential client for smaller requests or when concurrent is disabled
		client := github.NewClient(&cfg.GitHub)

		if *count <= 30 {
			users, err = client.FetchUsers(*count, *since)
		} else {
			users, err = client.FetchAllUsers(*count)
		}
	}

	fetchDuration := time.Since(startTime)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching users: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Fetched %d users in %v", len(users), fetchDuration)

	// Store users in database if requested
	if *store && len(users) > 0 {
		if err := userRepo.InsertUsers(users); err != nil {
			fmt.Fprintf(os.Stderr, "Error storing users: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Stored %d users in database\n", len(users))
	}

	// Display users
	if err := displayUsers(users, *format); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying users: %v\n", err)
		os.Exit(1)
	}
}

func listUsersCommand(cfg *config.Config) {
	// Create a new FlagSet for the list-users subcommand
	listUsersCmd := flag.NewFlagSet("list-users", flag.ExitOnError)

	var (
		limit  = listUsersCmd.Int("limit", 30, "Number of users to list")
		offset = listUsersCmd.Int("offset", 0, "Offset for pagination")
		format = listUsersCmd.String("format", "table", "Output format: table, json, detailed")
		help   = listUsersCmd.Bool("help", false, "Show help message")
	)

	listUsersCmd.Usage = func() {
		printListUsersUsage()
	}

	// Parse the arguments starting from position 2 (after list-users)
	listUsersCmd.Parse(os.Args[2:])

	if *help {
		printListUsersUsage()
		os.Exit(0)
	}

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	userRepo := database.NewUserRepository(db)

	// Get users from database
	users, err := userRepo.GetUsers(*limit, *offset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching users from database: %v\n", err)
		os.Exit(1)
	}

	// Get total count
	total, err := userRepo.GetTotalUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting total user count: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Showing %d of %d total users (offset: %d)\n\n", len(users), total, *offset)

	// Display users
	if err := displayUsers(users, *format); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying users: %v\n", err)
		os.Exit(1)
	}
}

func setupDBCommand(cfg *config.Config) {
	// Create a new FlagSet for the setup-db subcommand
	setupDBCmd := flag.NewFlagSet("setup-db", flag.ExitOnError)

	var (
		help = setupDBCmd.Bool("help", false, "Show help message")
	)

	setupDBCmd.Usage = func() {
		printSetupDBUsage()
	}

	// Parse the arguments starting from position 2 (after setup-db)
	setupDBCmd.Parse(os.Args[2:])

	if *help {
		printSetupDBUsage()
		os.Exit(0)
	}

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create schema
	if err := db.CreateSchema(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating database schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Database schema created successfully!")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "GitHub Users CLI Tool\n\n")
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  sync-users    Fetch users from GitHub API\n")
	fmt.Fprintf(os.Stderr, "  list-users    List users from database\n")
	fmt.Fprintf(os.Stderr, "  setup-db      Initialize database schema\n")
	fmt.Fprintf(os.Stderr, "  serve         Start REST API server\n")
	fmt.Fprintf(os.Stderr, "  help          Show this help message\n")
	fmt.Fprintf(os.Stderr, "\nUse '%s <command> --help' for more information about a command.\n", os.Args[0])
}

func printSyncUsersUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s sync-users [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Fetches users from GitHub Users REST API (v2022-11-28) with concurrent processing\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -count int\n")
	fmt.Fprintf(os.Stderr, "        Number of users to fetch (max 1000) (default 30)\n")
	fmt.Fprintf(os.Stderr, "  -concurrent\n")
	fmt.Fprintf(os.Stderr, "        Use concurrent fetching with worker pools (default true)\n")
	fmt.Fprintf(os.Stderr, "  -format string\n")
	fmt.Fprintf(os.Stderr, "        Output format: table, json, detailed (default \"table\")\n")
	fmt.Fprintf(os.Stderr, "  -since int\n")
	fmt.Fprintf(os.Stderr, "        User ID to start from (0 = auto-detect when storing) (default 0)\n")
	fmt.Fprintf(os.Stderr, "  -store\n")
	fmt.Fprintf(os.Stderr, "        Store users in MySQL database\n")
	fmt.Fprintf(os.Stderr, "  -help\n")
	fmt.Fprintf(os.Stderr, "        Show help message\n")
	fmt.Fprintf(os.Stderr, "\nConcurrency Configuration (via environment variables):\n")
	fmt.Fprintf(os.Stderr, "  GITHUB_MAX_WORKERS=5              Number of concurrent workers\n")
	fmt.Fprintf(os.Stderr, "  GITHUB_RATE_LIMIT_DELAY_MS=1000   Delay between API calls (ms)\n")
	fmt.Fprintf(os.Stderr, "  GITHUB_MAX_RETRIES=3              Max retries for failed requests\n")
	fmt.Fprintf(os.Stderr, "  GITHUB_RETRY_BACKOFF_MS=500       Base backoff time for retries (ms)\n")
}

func printListUsersUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s list-users [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Lists users from the MySQL database\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -limit int\n")
	fmt.Fprintf(os.Stderr, "        Number of users to display (default 30)\n")
	fmt.Fprintf(os.Stderr, "  -offset int\n")
	fmt.Fprintf(os.Stderr, "        Offset for pagination (default 0)\n")
	fmt.Fprintf(os.Stderr, "  -format string\n")
	fmt.Fprintf(os.Stderr, "        Output format: table, json, detailed (default \"table\")\n")
	fmt.Fprintf(os.Stderr, "  -help\n")
	fmt.Fprintf(os.Stderr, "        Show help message\n")
}

func printSetupDBUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s setup-db\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Initializes the MySQL database schema for storing GitHub users\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -help\n")
	fmt.Fprintf(os.Stderr, "        Show help message\n")
}

func displayUsers(users []github.User, format string) error {
	switch format {
	case "json":
		return github.DisplayUsersJSON(users)
	case "table":
		return github.DisplayUsersTable(users)
	case "detailed":
		return github.DisplayUsersDetailed(users)
	default:
		return fmt.Errorf("unsupported format: %s (supported: table, json, detailed)", format)
	}
}
