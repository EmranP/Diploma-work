package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const AccessTokenTTL = 8 * time.Hour

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func LoginHandler(db *gorm.DB, secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body LoginRequest

		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(
				http.StatusBadRequest,
				"Invalid request body",
			)
		}

		email := strings.ToLower(strings.TrimSpace(body.Email))

		if email == "" || body.Password == "" {
			return fiber.NewError(
				http.StatusBadRequest,
				"Email and password are required",
			)
		}

		var account infra.UserModel

		err := db.WithContext(c.UserContext()).
			Where("email = ?", email).
			First(&account).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Email or password is incorrect",
			)
		}

		if err != nil {
			return err
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(account.PasswordHash),
			[]byte(body.Password),
		) != nil {
			return fiber.NewError(
				http.StatusUnauthorized,
				"Email or password is incorrect",
			)
		}

		role := normalizeRole(account.Role)

		if !isValidRole(role) {
			return fiber.NewError(
				http.StatusInternalServerError,
				"Account role is not configured correctly",
			)
		}

		now := time.Now()

		claims := AccessClaims{
			Role: role,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    TokenIssuer,
				Subject:   strconv.FormatUint(uint64(account.ID), 10),
				Audience:  jwt.ClaimStrings{TokenAudience},
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			},
		}

		token := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			claims,
		)

		signedToken, err := token.SignedString([]byte(secret))
		if err != nil {
			return err
		}

		return c.JSON(fiber.Map{
			"accessToken": signedToken,
			"tokenType":   "Bearer",
			"expiresIn":   int(AccessTokenTTL.Seconds()),
			"user": fiber.Map{
				"id":    account.ID,
				"email": account.Email,
				"role":  role,
			},
		})
	}
}
