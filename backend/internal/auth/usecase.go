package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const (
	LocalUserIDKey   = "userID"
	LocalUserRoleKey = "userRole"
)

func AuthMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)

		const prefix = "Bearer "

		if len(header) < len(prefix) ||
			!strings.EqualFold(header[:len(prefix)], prefix) {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Bearer token is required",
			)
		}

		rawToken := strings.TrimSpace(header[len(prefix):])

		if rawToken == "" || strings.ContainsAny(rawToken, " \t\r\n") {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Invalid token",
			)
		}

		claims := new(AccessClaims)

		token, err := jwt.ParseWithClaims(
			rawToken,
			claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, errors.New("unexpected signing method")
				}

				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{
				jwt.SigningMethodHS256.Alg(),
			}),
			jwt.WithIssuer(TokenIssuer),
			jwt.WithAudience(TokenAudience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
		)

		if err != nil || token == nil || !token.Valid {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Invalid or expired token",
			)
		}

		parsedClaims, ok := token.Claims.(*AccessClaims)
		if !ok {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Invalid token claims",
			)
		}

		id, err := strconv.ParseUint(
			parsedClaims.Subject,
			10,
			64,
		)

		if err != nil || id == 0 || uint64(uint(id)) != id {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Invalid token subject",
			)
		}

		role := normalizeRole(parsedClaims.Role)

		if !isValidRole(role) {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Invalid token role",
			)
		}

		c.Locals(LocalUserIDKey, uint(id))
		c.Locals(LocalUserRoleKey, role)

		return c.Next()
	}
}
