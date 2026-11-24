package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
)

func signClaimsForTest(
	t *testing.T,
	claims auth.AccessClaims,
	secret string,
) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	return signed
}

func validClaimsForTest() auth.AccessClaims {
	now := time.Now()

	return auth.AccessClaims{
		Role: auth.RoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    auth.TokenIssuer,
			Subject:   "42",
			Audience:  jwt.ClaimStrings{auth.TokenAudience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
}

func protectedAppForTest() *fiber.App {
	app := fiber.New()

	app.Get(
		"/protected",
		auth.AuthMiddleware(testSecret),
		func(c *fiber.Ctx) error {
			return c.SendStatus(http.StatusNoContent)
		},
	)

	return app
}

func requestStatusForTest(
	t *testing.T,
	app *fiber.App,
	authorization string,
) int {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func TestAuthMiddlewareRejectsInvalidAuthorizationHeaders(t *testing.T) {
	logTestStep(t, "отклонение некорректных заголовков Authorization в AuthMiddleware")

	tests := []struct {
		name          string
		authorization string
	}{
		{
			name:          "missing authorization",
			authorization: "",
		},
		{
			name:          "basic authentication",
			authorization: "Basic credentials",
		},
		{
			name:          "bearer without token",
			authorization: "Bearer",
		},
		{
			name:          "empty bearer token",
			authorization: "Bearer ",
		},
		{
			name:          "token contains spaces",
			authorization: "Bearer invalid token",
		},
		{
			name:          "malformed JWT",
			authorization: "Bearer invalid.token.value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			app := protectedAppForTest()

			status := requestStatusForTest(
				t,
				app,
				tt.authorization,
			)

			if status != http.StatusUnauthorized {
				t.Fatalf(
					"expected 401, got %d",
					status,
				)
			}
		})
	}
}

func TestAuthMiddlewareRejectsInvalidRegisteredClaims(t *testing.T) {
	logTestStep(t, "отклонение токенов с невалидными стандартными JWT claims")

	tests := []struct {
		name   string
		mutate func(*auth.AccessClaims)
	}{
		{
			name: "wrong issuer",
			mutate: func(claims *auth.AccessClaims) {
				claims.Issuer = "another-issuer"
			},
		},
		{
			name: "wrong audience",
			mutate: func(claims *auth.AccessClaims) {
				claims.Audience = jwt.ClaimStrings{"another-api"}
			},
		},
		{
			name: "future not before",
			mutate: func(claims *auth.AccessClaims) {
				claims.NotBefore = jwt.NewNumericDate(
					time.Now().Add(time.Hour),
				)
			},
		},
		{
			name: "future issued at",
			mutate: func(claims *auth.AccessClaims) {
				claims.IssuedAt = jwt.NewNumericDate(
					time.Now().Add(time.Hour),
				)
			},
		},
		{
			name: "missing expiration",
			mutate: func(claims *auth.AccessClaims) {
				claims.ExpiresAt = nil
			},
		},
		{
			name: "empty subject",
			mutate: func(claims *auth.AccessClaims) {
				claims.Subject = ""
			},
		},
		{
			name: "zero subject",
			mutate: func(claims *auth.AccessClaims) {
				claims.Subject = "0"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			claims := validClaimsForTest()
			tt.mutate(&claims)

			token := signClaimsForTest(
				t,
				claims,
				testSecret,
			)

			app := protectedAppForTest()

			status := requestStatusForTest(
				t,
				app,
				"Bearer "+token,
			)

			if status != http.StatusUnauthorized {
				t.Fatalf(
					"expected 401, got %d",
					status,
				)
			}
		})
	}
}

func TestAuthMiddlewareRejectsInvalidSignature(t *testing.T) {
	logTestStep(t, "отклонение JWT-токена с неверной подписью")

	claims := validClaimsForTest()

	token := signClaimsForTest(
		t,
		claims,
		"another-secret-that-is-not-used-by-the-server",
	)

	app := protectedAppForTest()

	status := requestStatusForTest(
		t,
		app,
		"Bearer "+token,
	)

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}
}

func TestAuthMiddlewareAcceptsCaseInsensitiveBearerPrefix(t *testing.T) {
	logTestStep(t, "прием префикса bearer в любом регистре")

	token := createTestToken(
		t,
		jwt.SigningMethodHS256,
		"42",
		auth.RoleUser,
		time.Now().Add(time.Hour),
	)

	app := protectedAppForTest()

	status := requestStatusForTest(
		t,
		app,
		"bearer "+token,
	)

	if status != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", status)
	}
}
