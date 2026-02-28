package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/spf13/cobra"
)

var (
	email    string
	password string
	isAdmin  bool
	dbName   string
	tursoURL string
	tursoToken string
)

var rootCmd = &cobra.Command{
	Use:   "tribble-admin",
	Short: "Admin CLI for ideal-tribble",
	Long:  `A command-line tool for administrative tasks like user management.`,
}

var createUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Create a new user",
	Long: `Create a new user in the database.

Example:
  tribble-admin create-user --email admin@example.com --password secret123 --admin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return createUser()
	},
}

var generateSecretCmd = &cobra.Command{
	Use:   "generate-secret",
	Short: "Generate a secure random secret",
	Long: `Generate a cryptographically secure random secret for use as
SESSION_SECRET or TOTP_ENCRYPTION_KEY.

Example:
  tribble-admin generate-secret
  tribble-admin generate-secret --length 32`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateSecret()
	},
}

var listUsersCmd = &cobra.Command{
	Use:   "list-users",
	Short: "List all users in the database",
	Long:  `List all users to verify database connectivity and user existence.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listUsers()
	},
}

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset a user's password",
	Long: `Reset the password for an existing user.

Example:
  tribble-admin reset-password --email admin@example.com --password newpassword123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return resetPassword()
	},
}

var secretLength int

func init() {
	// Load .env file
	_ = godotenv.Load()

	// Database flags
	rootCmd.PersistentFlags().StringVar(&dbName, "db", os.Getenv("DB_NAME"), "Database name")
	rootCmd.PersistentFlags().StringVar(&tursoURL, "turso-url", os.Getenv("TURSO_PRIMARY_URL"), "Turso database URL")
	rootCmd.PersistentFlags().StringVar(&tursoToken, "turso-token", os.Getenv("TURSO_AUTH_TOKEN"), "Turso auth token")

	// Create user flags
	createUserCmd.Flags().StringVar(&email, "email", "", "User email address (required)")
	createUserCmd.Flags().StringVar(&password, "password", "", "User password (required, min 8 chars)")
	createUserCmd.Flags().BoolVar(&isAdmin, "admin", false, "Make user an administrator")
	createUserCmd.MarkFlagRequired("email")
	createUserCmd.MarkFlagRequired("password")

	// Generate secret flags
	generateSecretCmd.Flags().IntVar(&secretLength, "length", 32, "Secret length in bytes")

	// Reset password flags
	resetPasswordCmd.Flags().StringVar(&email, "email", "", "User email address (required)")
	resetPasswordCmd.Flags().StringVar(&password, "password", "", "New password (required, min 8 chars)")
	resetPasswordCmd.MarkFlagRequired("email")
	resetPasswordCmd.MarkFlagRequired("password")

	rootCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(generateSecretCmd)
	rootCmd.AddCommand(listUsersCmd)
	rootCmd.AddCommand(resetPasswordCmd)
}

func main() {
	log.SetFormatter(log.TextFormatter)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createUser() error {
	// Validate inputs
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Initialize database
	db, teardown, err := database.InitDB(dbName, tursoURL, tursoToken, "./migrations")
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer teardown()

	// Create auth store
	authStore := auth.NewStore(db)

	// Check if user already exists
	existingUser, _ := authStore.GetUserByEmail(email)
	if existingUser != nil {
		return fmt.Errorf("user with email %s already exists", email)
	}

	// Create user
	user, err := authStore.CreateUser(email, password, isAdmin)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	role := "user"
	if isAdmin {
		role = "admin"
	}

	fmt.Printf("✓ User created successfully!\n")
	fmt.Printf("  ID:    %d\n", user.ID)
	fmt.Printf("  Email: %s\n", user.Email)
	fmt.Printf("  Role:  %s\n", role)
	fmt.Printf("\nYou can now log in at /login with these credentials.\n")

	return nil
}

func generateSecret() error {
	bytes := make([]byte, secretLength)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	secret := base64.URLEncoding.EncodeToString(bytes)

	fmt.Printf("Generated secret (%d bytes, base64 encoded):\n", secretLength)
	fmt.Printf("  %s\n", secret)
	fmt.Printf("\nFor 32-byte secrets (AES-256), use the first 32 characters:\n")
	if len(secret) >= 32 {
		fmt.Printf("  %s\n", secret[:32])
	}

	return nil
}

func listUsers() error {
	// Initialize database
	db, teardown, err := database.InitDB(dbName, tursoURL, tursoToken, "./migrations")
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer teardown()

	// Create auth store
	authStore := auth.NewStore(db)

	users, err := authStore.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No users found in the database.")
		return nil
	}

	fmt.Printf("Found %d user(s):\n\n", len(users))
	for _, user := range users {
		role := "user"
		if user.IsAdmin {
			role = "admin"
		}
		totp := "disabled"
		if user.TOTPEnabled {
			totp = "enabled"
		}
		fmt.Printf("  ID: %d\n", user.ID)
		fmt.Printf("  Email: %s\n", user.Email)
		fmt.Printf("  Role: %s\n", role)
		fmt.Printf("  TOTP: %s\n", totp)
		fmt.Printf("  Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	return nil
}

func resetPassword() error {
	// Validate inputs
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Initialize database
	db, teardown, err := database.InitDB(dbName, tursoURL, tursoToken, "./migrations")
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer teardown()

	// Create auth store
	authStore := auth.NewStore(db)

	// Find user
	user, err := authStore.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Update password
	if err := authStore.UpdatePassword(user.ID, password); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	fmt.Printf("✓ Password reset successfully for %s\n", email)
	return nil
}
