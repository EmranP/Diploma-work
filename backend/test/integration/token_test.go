package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
)

const testSecret = "01234567890123456789012345678901"
const integrationJWTSecret = "integration-test-secret-32-bytes-minimum"

func createTestToken(
	t *testing.T,
	method jwt.SigningMethod,
	subject string,
	role string,
	expiresAt time.Time,
) string {
	t.Helper()

	now := time.Now()

	claims := auth.AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    auth.TokenIssuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{auth.TokenAudience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(method, claims)

	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}

	return signed
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	logTestStep(t, "пропуск валидного JWT-токена и передача userID в контекст")

	app := fiber.New()

	app.Get(
		"/protected",
		auth.AuthMiddleware(testSecret),
		func(c *fiber.Ctx) error {
			id, err := auth.UserIDFromContext(c)
			if err != nil {
				return err
			}

			return c.SendString(
				strconv.FormatUint(uint64(id), 10),
			)
		},
	)

	token := createTestToken(
		t,
		jwt.SigningMethodHS256,
		"42",
		auth.RoleUser,
		time.Now().Add(time.Hour),
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthMiddlewareRejectsInvalidTokens(t *testing.T) {
	logTestStep(t, "отклонение просроченных, некорректных и подписанных другим алгоритмом токенов")

	now := time.Now()

	tests := []struct {
		name   string
		method jwt.SigningMethod
		sub    string
		role   string
		exp    time.Time
	}{
		{
			name:   "expired token",
			method: jwt.SigningMethodHS256,
			sub:    "1",
			role:   auth.RoleUser,
			exp:    now.Add(-time.Hour),
		},
		{
			name:   "invalid subject",
			method: jwt.SigningMethodHS256,
			sub:    "invalid",
			role:   auth.RoleUser,
			exp:    now.Add(time.Hour),
		},
		{
			name:   "invalid role",
			method: jwt.SigningMethodHS256,
			sub:    "1",
			role:   "superuser",
			exp:    now.Add(time.Hour),
		},
		{
			name:   "unexpected signing algorithm",
			method: jwt.SigningMethodHS384,
			sub:    "1",
			role:   auth.RoleUser,
			exp:    now.Add(time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			app := fiber.New()

			app.Get(
				"/protected",
				auth.AuthMiddleware(testSecret),
				func(c *fiber.Ctx) error {
					return c.SendStatus(http.StatusOK)
				},
			)

			token := createTestToken(
				t,
				tt.method,
				tt.sub,
				tt.role,
				tt.exp,
			)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf(
					"expected 401, got %d",
					resp.StatusCode,
				)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	logTestStep(t, "проверка доступа по роли через AuthMiddleware + RequireRole")

	app := fiber.New()

	app.Get(
		"/admin",
		auth.AuthMiddleware(testSecret),
		auth.RequireRole(auth.RoleAdmin),
		func(c *fiber.Ctx) error {
			return c.SendStatus(http.StatusNoContent)
		},
	)

	tests := []struct {
		role           string
		expectedStatus int
	}{
		{
			role:           auth.RoleUser,
			expectedStatus: http.StatusForbidden,
		},
		{
			role:           auth.RoleAdmin,
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			logTestStep(t, "кейс роль: "+tt.role)

			token := createTestToken(
				t,
				jwt.SigningMethodHS256,
				"1",
				tt.role,
				time.Now().Add(time.Hour),
			)

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Fatalf(
					"expected %d, got %d",
					tt.expectedStatus,
					resp.StatusCode,
				)
			}
		})
	}
}

func TestValidateJWTSecret(t *testing.T) {
	logTestStep(t, "валидация длины и наличия JWT секрета")

	tests := []struct {
		secret string
		valid  bool
	}{
		{secret: testSecret, valid: true},
		{secret: "short", valid: false},
		{secret: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("valid_%t", tt.valid), func(t *testing.T) {
			logTestStep(t, fmt.Sprintf("кейс valid=%t", tt.valid))

			err := auth.ValidateJWTSecret(tt.secret)

			if tt.valid && err != nil {
				t.Fatalf("expected valid secret, got %v", err)
			}

			if !tt.valid && err == nil {
				t.Fatal("expected secret validation error")
			}
		})
	}
}
