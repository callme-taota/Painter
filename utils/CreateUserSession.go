package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// secureRandomString generates a secure random string using a seed and random salt.
func secureRandomString(seed string) (string, error) {
	// Generate a random salt.
	salt, err := generateRandomSalt()
	if err != nil {
		return "", err
	}

	// Combine seed and salt, then hash using SHA-256.
	seedBytes := []byte(seed)
	combinedBytes := append(seedBytes, salt...)
	hash := sha256.Sum256(combinedBytes)

	// Convert the hash to a hexadecimal string.
	hexString := hex.EncodeToString(hash[:])
	return hexString, nil
}

// generateRandomSalt generates a random salt of 16 bytes.
func generateRandomSalt() ([]byte, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, errors.New("unable to generate random salt")
	}

	return salt, nil
}

// CreateSession generates a secure session using the provided ID.
func CreateSession(id string) (string, error) {
	// Generate a secure random string based on the ID.
	secureRandomStr, err := secureRandomString(fmt.Sprintf("%s", id))
	if err != nil {
		return "", err
	}
	return secureRandomStr, nil
}
