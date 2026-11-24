package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenIssuer   = "phishing-detector"
	TokenAudience = "phishing-detector-api"

	RoleAdmin = "admin"
	RoleUser  = "user"
)

const MinimumJWTSecretLength = 32

type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func ValidateJWTSecret(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("JWT secret is empty")
	}

	if len(secret) < MinimumJWTSecretLength {
		return errors.New("JWT secret must contain at least 32 bytes")
	}

	return nil
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func isValidRole(role string) bool {
	switch normalizeRole(role) {
	case RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}
