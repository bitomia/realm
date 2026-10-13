package golang

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func authTokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}
	return filepath.Join(homeDir, ".realmrc"), nil
}

func getAuthToken() string {
	// First check environment variable for backwards compatibility
	if bearer, exists := os.LookupEnv("REALM_BEARER"); exists {
		return bearer
	}

	// Then check ~/.realmrc file
	realmrcPath, err := authTokenPath()
	if err != nil {
		return ""
	}

	tokenBytes, err := os.ReadFile(realmrcPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(tokenBytes))
}

// SaveAuthToken stores the authentication token in ~/.realmr
func SaveAuthToken(token string) error {
	realmrcPath, err := authTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(realmrcPath, []byte(token), 0600)
}

// HasAuthToken returns true if a token is stored in ~/.realmrc
func HasAuthToken() (bool, error) {
	realmrcPath, err := authTokenPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(realmrcPath); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// RemoveAuthToken removes the token stored in ~/.realmrc, removing the session.
func RemoveAuthToken() error {
	realmrcPath, err := authTokenPath()
	if err != nil {
		return err
	}
	return os.Remove(realmrcPath)
}
