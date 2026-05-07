package db

import (
	"encoding/json"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     int32  `json:"role"`
}

func (db *AgentDB) GetVerifiedUser(username string, password string) (int32, error) {
	slog.Info("Login request", "username", username)
	userKey, err := db.userKey(username)
	if err != nil {
		slog.Error("Error getting user key", "error", err.Error())
		return -1, err
	}

	value, err := db.get(userKey)
	if err != nil {
		slog.Error("Error on GetVerifiedUser", "error", err.Error())
		return -1, nil // User not found
	}

	var user User
	if err := json.Unmarshal([]byte(value), &user); err != nil {
		slog.Error("Error unmarshaling user", "error", err.Error())
		return -1, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return -1, err
	}
	return user.Role, nil
}

func (db *AgentDB) CreateUser(username string, password string, role int32) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	user := User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
	}

	value, err := json.Marshal(user)
	if err != nil {
		slog.Error("Error marshaling user", "error", err.Error())
		return err
	}

	userKey, err := db.userKey(username)
	if err != nil {
		slog.Error("Error getting user key", "error", err.Error())
		return err
	}

	return db.put(userKey, string(value))
}
