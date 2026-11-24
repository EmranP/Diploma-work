package auth

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

var ErrUserIdentityNotFound = errors.New("authenticated user identity not found")

func UserIDFromContext(c *fiber.Ctx) (uint, error) {
	value := c.Locals(LocalUserIDKey)

	id, ok := value.(uint)
	if !ok || id == 0 {
		return 0, ErrUserIdentityNotFound
	}

	return id, nil
}

func UserRoleFromContext(c *fiber.Ctx) (string, error) {
	value := c.Locals(LocalUserRoleKey)

	role, ok := value.(string)
	if !ok {
		return "", ErrUserIdentityNotFound
	}

	role = normalizeRole(role)

	if !isValidRole(role) {
		return "", ErrUserIdentityNotFound
	}

	return role, nil
}

func RequireRole(requiredRoles ...string) fiber.Handler {
	allowedRoles := make(map[string]struct{}, len(requiredRoles))

	for _, role := range requiredRoles {
		normalized := normalizeRole(role)

		if isValidRole(normalized) {
			allowedRoles[normalized] = struct{}{}
		}
	}

	return func(c *fiber.Ctx) error {
		role, err := UserRoleFromContext(c)
		if err != nil {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Authentication required",
			)
		}

		if _, allowed := allowedRoles[role]; !allowed {
			return fiber.NewError(
				http.StatusForbidden,
				"Insufficient permissions",
			)
		}

		return c.Next()
	}
}
