package integration_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
)

func TestUserIDFromContext(t *testing.T) {
	logTestStep(t, "извлечение userID из контекста Fiber")

	tests := []struct {
		name    string
		value   any
		set     bool
		want    uint
		wantErr bool
	}{
		{
			name:  "valid user ID",
			value: uint(42),
			set:   true,
			want:  42,
		},
		{
			name:    "missing identity",
			wantErr: true,
		},
		{
			name:    "zero user ID",
			value:   uint(0),
			set:     true,
			wantErr: true,
		},
		{
			name:    "wrong identity type",
			value:   int(42),
			set:     true,
			wantErr: true,
		},
		{
			name:    "nil identity",
			value:   nil,
			set:     true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			app := fiber.New()

			app.Get("/", func(c *fiber.Ctx) error {
				if tt.set {
					c.Locals(auth.LocalUserIDKey, tt.value)
				}

				got, err := auth.UserIDFromContext(c)

				if tt.wantErr {
					if !errors.Is(err, auth.ErrUserIdentityNotFound) {
						t.Errorf("expected ErrUserIdentityNotFound, got %v", err)
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}

					if got != tt.want {
						t.Errorf("expected user ID %d, got %d", tt.want, got)
					}
				}

				return c.SendStatus(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", resp.StatusCode)
			}
		})
	}
}

func TestUserRoleFromContext(t *testing.T) {
	logTestStep(t, "извлечение и нормализация роли пользователя из контекста Fiber")

	tests := []struct {
		name    string
		value   any
		set     bool
		want    string
		wantErr bool
	}{
		{
			name:  "admin role",
			value: auth.RoleAdmin,
			set:   true,
			want:  auth.RoleAdmin,
		},
		{
			name:  "user role",
			value: auth.RoleUser,
			set:   true,
			want:  auth.RoleUser,
		},
		{
			name:  "normalized role",
			value: " ADMIN ",
			set:   true,
			want:  auth.RoleAdmin,
		},
		{
			name:    "missing role",
			wantErr: true,
		},
		{
			name:    "invalid role",
			value:   "superuser",
			set:     true,
			wantErr: true,
		},
		{
			name:    "empty role",
			value:   "",
			set:     true,
			wantErr: true,
		},
		{
			name:    "wrong role type",
			value:   123,
			set:     true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			app := fiber.New()

			app.Get("/", func(c *fiber.Ctx) error {
				if tt.set {
					c.Locals(auth.LocalUserRoleKey, tt.value)
				}

				got, err := auth.UserRoleFromContext(c)

				if tt.wantErr {
					if !errors.Is(err, auth.ErrUserIdentityNotFound) {
						t.Errorf("expected ErrUserIdentityNotFound, got %v", err)
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}

					if got != tt.want {
						t.Errorf("expected role %q, got %q", tt.want, got)
					}
				}

				return c.SendStatus(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", resp.StatusCode)
			}
		})
	}
}

func TestRequireRoleWithoutValidIdentity(t *testing.T) {
	logTestStep(t, "проверка middleware RequireRole для валидных и невалидных ролей")

	tests := []struct {
		name           string
		role           any
		set            bool
		expectedStatus int
	}{
		{
			name:           "missing role",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid role",
			role:           "superuser",
			set:            true,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "wrong role type",
			role:           123,
			set:            true,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "user is forbidden",
			role:           auth.RoleUser,
			set:            true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "admin is allowed",
			role:           auth.RoleAdmin,
			set:            true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "normalized admin is allowed",
			role:           " ADMIN ",
			set:            true,
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			app := fiber.New()

			app.Get(
				"/admin",
				func(c *fiber.Ctx) error {
					if tt.set {
						c.Locals(auth.LocalUserRoleKey, tt.role)
					}

					return c.Next()
				},
				auth.RequireRole(auth.RoleAdmin),
				func(c *fiber.Ctx) error {
					return c.SendStatus(http.StatusNoContent)
				},
			)

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.expectedStatus,
					resp.StatusCode,
				)
			}
		})
	}
}
