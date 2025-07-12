package models

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// GenerateRandomPassword generates a random password of specified length
func GenerateRandomPassword(length int) (string, error) {
	if length < 4 {
		length = 6 // Default to 6 digits minimum
	}

	// Generate random digits
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}

	// Convert to digits (0-9)
	password := ""
	for i := 0; i < length; i++ {
		password += fmt.Sprintf("%d", bytes[i]%10)
	}

	return password, nil
}

// HashPassword creates a bcrypt hash of the password for Traefik
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// GetTraefikContainerName returns the Docker container name for a hostname's Traefik instance
func GetTraefikContainerName(hostname string) string {
	return fmt.Sprintf("tunnelman-traefik-%s", hostname)
}

// GetTraefikServiceURL returns the service URL that the tunnel should point to when auth is enabled
func GetTraefikServiceURL(hostname string, port int) string {
	return fmt.Sprintf("http://localhost:%d", port)
}
