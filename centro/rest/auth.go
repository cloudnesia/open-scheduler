package rest

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	Username string
	// PasswordHash stores bcrypt hashed password
	PasswordHash string
	Role         string
	CreatedAt    int64
}

// UserManager manages user authentication and authorization
type UserManager struct {
	users map[string]*User
	mu    sync.RWMutex
}

var userManager *UserManager

func init() {
	userManager = NewUserManager()
	// Initialize with default admin user
	// Default password: admin123 (MUST be changed in production)
	defaultPassword := "admin123"
	if err := userManager.CreateUser("admin", defaultPassword, "admin"); err != nil {
		log.Printf("[Centro Auth] Failed to create default admin user: %v", err)
	}
}

// NewUserManager creates a new user manager instance
func NewUserManager() *UserManager {
	return &UserManager{
		users: make(map[string]*User),
	}
}

// CreateUser creates a new user with bcrypt hashed password
func (um *UserManager) CreateUser(username, password, role string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Validate input
	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	// Check if user already exists
	if _, exists := um.users[username]; exists {
		return fmt.Errorf("user already exists")
	}

	// Hash password with bcrypt (cost=10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	um.users[username] = &User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         role,
	}

	log.Printf("[Centro Auth] User created: %s (role: %s)", username, role)
	return nil
}

// VerifyCredentials verifies username and password, returns user if valid
func (um *UserManager) VerifyCredentials(username, password string) (*User, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	if !exists {
		// Use dummy hash to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$10$N9qo8ucoathceehololgod"), []byte(password))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Compare password with hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

// ChangePassword changes a user's password
func (um *UserManager) ChangePassword(username, oldPassword, newPassword string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[username]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("invalid current password")
	}

	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters long")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	log.Printf("[Centro Auth] Password changed for user: %s", username)
	return nil
}

// GetUser retrieves a user by username
func (um *UserManager) GetUser(username string) (*User, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// ListUsers returns all usernames (for admin purposes)
func (um *UserManager) ListUsers() []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	usernames := make([]string, 0, len(um.users))
	for username := range um.users {
		usernames = append(usernames, username)
	}
	return usernames
}

// DeleteUser deletes a user (admin only)
func (um *UserManager) DeleteUser(username string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	if username == "admin" {
		return fmt.Errorf("cannot delete admin user")
	}

	if _, exists := um.users[username]; !exists {
		return fmt.Errorf("user not found")
	}

	delete(um.users, username)
	log.Printf("[Centro Auth] User deleted: %s", username)
	return nil
}

// GetUserManager returns the global user manager instance
func GetUserManager() *UserManager {
	return userManager
}
