package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	authpkg "github.com/patdyyew-emran/phishing-detector/internal/auth"
	"gorm.io/gorm"
)

const testLoginPassword = "correct-password-123"

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int    `json:"expiresIn"`

	User struct {
		ID    uint   `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
}

func createLoginAccount(
	t *testing.T,
	db *gorm.DB,
	role string,
) (string, uint) {
	t.Helper()

	email := fmt.Sprintf(
		"login-%d@example.test",
		time.Now().UnixNano(),
	)

	user := createTestUser(
		t,
		db,
		email,
		testLoginPassword,
		role,
	)

	return email, user.ID
}

func TestLoginSuccess(t *testing.T) {
	logTestStep(t, "успешный логин пользователя и проверка JWT claims")

	db := openTestDB(t)

	email, userID := createLoginAccount(
		t,
		db,
		authpkg.RoleUser,
	)

	router := newTestRouter(db)

	body := fmt.Sprintf(
		`{"email":%q,"password":%q}`,
		" "+strings.ToUpper(email)+" ",
		testLoginPassword,
	)

	resp, responseBody := performRequest(
		t,
		router,
		http.MethodPost,
		"",
		"/api/auth/login",
		body,
	)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			resp.StatusCode,
			responseBody,
		)
	}

	var result loginResponse

	if err := json.Unmarshal(responseBody, &result); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	if result.TokenType != "Bearer" {
		t.Fatalf(
			"expected token type Bearer, got %q",
			result.TokenType,
		)
	}

	if result.ExpiresIn != int(authpkg.AccessTokenTTL.Seconds()) {
		t.Fatalf(
			"unexpected expiresIn: %d",
			result.ExpiresIn,
		)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"expected user ID %d, got %d",
			userID,
			result.User.ID,
		)
	}

	if result.User.Email != email {
		t.Fatalf(
			"expected email %q, got %q",
			email,
			result.User.Email,
		)
	}

	if result.User.Role != authpkg.RoleUser {
		t.Fatalf(
			"expected role %q, got %q",
			authpkg.RoleUser,
			result.User.Role,
		)
	}

	claims := new(authpkg.AccessClaims)

	parsedToken, err := jwt.ParseWithClaims(
		result.AccessToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf(
					"unexpected signing method: %s",
					token.Method.Alg(),
				)
			}

			return []byte(integrationJWTSecret), nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(authpkg.TokenIssuer),
		jwt.WithAudience(authpkg.TokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)

	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if !parsedToken.Valid {
		t.Fatal("access token is invalid")
	}

	tokenClaims, ok := parsedToken.Claims.(*authpkg.AccessClaims)
	if !ok {
		t.Fatal("unexpected token claims type")
	}

	expectedSubject := strconv.FormatUint(uint64(userID), 10)

	if tokenClaims.Subject != expectedSubject {
		t.Fatalf(
			"expected subject %q, got %q",
			expectedSubject,
			tokenClaims.Subject,
		)
	}

	if tokenClaims.Role != authpkg.RoleUser {
		t.Fatalf(
			"expected token role %q, got %q",
			authpkg.RoleUser,
			tokenClaims.Role,
		)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	logTestStep(t, "отклонение логина при неверном пароле или email")

	db := openTestDB(t)

	email, _ := createLoginAccount(
		t,
		db,
		authpkg.RoleUser,
	)

	router := newTestRouter(db)

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "wrong password",
			email:    email,
			password: "wrong-password",
		},
		{
			name:     "unknown email",
			email:    "unknown@example.test",
			password: testLoginPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			body := fmt.Sprintf(
				`{"email":%q,"password":%q}`,
				tt.email,
				tt.password,
			)

			resp, responseBody := performRequest(
				t,
				router,
				http.MethodPost,
				"",
				"/api/auth/login",
				body,
			)

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf(
					"expected status 401, got %d: %s",
					resp.StatusCode,
					responseBody,
				)
			}
		})
	}
}

func TestLoginRejectsInvalidRequestBody(t *testing.T) {
	logTestStep(t, "отклонение некорректного JSON или пустых обязательных полей при логине")

	db := openTestDB(t)
	router := newTestRouter(db)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed JSON",
			body: `{`,
		},
		{
			name: "empty object",
			body: `{}`,
		},
		{
			name: "missing email",
			body: `{"password":"password123"}`,
		},
		{
			name: "missing password",
			body: `{"email":"user@example.test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logTestStep(t, "кейс: "+tt.name)

			resp, responseBody := performRequest(
				t,
				router,
				http.MethodPost,
				"",
				"/api/auth/login",
				tt.body,
			)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf(
					"expected status 400, got %d: %s",
					resp.StatusCode,
					responseBody,
				)
			}
		})
	}
}

func TestLoginRejectsInvalidAccountRole(t *testing.T) {
	logTestStep(t, "отклонение логина для аккаунта с недопустимой ролью")

	db := openTestDB(t)

	email, _ := createLoginAccount(
		t,
		db,
		"superuser",
	)

	router := newTestRouter(db)

	body := fmt.Sprintf(
		`{"email":%q,"password":%q}`,
		email,
		testLoginPassword,
	)

	resp, responseBody := performRequest(
		t,
		router,
		http.MethodPost,
		"",
		"/api/auth/login",
		body,
	)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf(
			"expected status 500, got %d: %s",
			resp.StatusCode,
			responseBody,
		)
	}
}
