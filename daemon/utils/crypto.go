package utils

import (
	"crypto/rand"
)

func GenerateKey(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789#-_"
	charsetLength := len(charset)

	password := make([]byte, length)
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	for i, b := range randomBytes {
		password[i] = charset[b%byte(charsetLength)]
	}
	return string(password), nil
}
