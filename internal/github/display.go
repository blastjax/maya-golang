package github

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// DisplayUsersJSON displays users in JSON format
func DisplayUsersJSON(users []User) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(users)
}

// DisplayUsersTable displays users in a formatted table
func DisplayUsersTable(users []User) error {
	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}

	// Create a tab writer for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, "ID\tLOGIN\tTYPE\tSITE_ADMIN\tURL")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	// Print each user
	for _, user := range users {
		fmt.Fprintf(w, "%d\t%s\t%s\n",
			user.ID,
			user.Login,
			user.Type,
		)
	}

	// Print summary
	fmt.Printf("\nTotal users fetched: %d\n", len(users))

	if len(users) > 0 {
		fmt.Printf("Next since value: %d\n", users[len(users)-1].ID)
	}

	return nil
}

// DisplayUsersDetailed displays users with more detailed information
func DisplayUsersDetailed(users []User) error {
	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}

	for i, user := range users {
		if i > 0 {
			fmt.Println(strings.Repeat("-", 50))
		}

		fmt.Printf("ID: %d\n", user.ID)
		fmt.Printf("Login: %s\n", user.Login)
		fmt.Printf("Type: %s\n", user.Type)
		fmt.Printf("Avatar URL: %s\n", user.AvatarURL)

		// Display optional fields if available
		if user.Name != nil {
			fmt.Printf("Name: %s\n", *user.Name)
		}
		if user.Company != nil {
			fmt.Printf("Company: %s\n", *user.Company)
		}
		if user.Location != nil {
			fmt.Printf("Location: %s\n", *user.Location)
		}
		if user.Bio != nil {
			fmt.Printf("Bio: %s\n", *user.Bio)
		}

		fmt.Println()
	}

	fmt.Printf("Total users fetched: %d\n", len(users))
	if len(users) > 0 {
		fmt.Printf("Next since value: %d\n", users[len(users)-1].ID)
	}

	return nil
}
