// ./local-proxy/pkg/utils/crypto.go
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword хеширует пароль
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// VerifyPassword проверяет пароль
func VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateRandomString генерирует случайную строку
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random string: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateAPIKey генерирует API ключ
func GenerateAPIKey() (string, error) {
	key, err := GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	return "sk_" + key, nil
}
