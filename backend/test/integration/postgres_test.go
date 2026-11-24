package integration_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/patdyyew-emran/phishing-detector/internal/app"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func logTestStep(t *testing.T, step string) {
	t.Helper()
	t.Logf("▶ [%s] %s", t.Name(), step)
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	err := godotenv.Load("../../.env")
	if err != nil {
		log.Printf("%s", err.Error())
	}

	dsn := os.Getenv("DB_TEST_URL")
	if dsn == "" {
		t.Skip("DB_TEST_URL from test is not configured")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	var databaseName string

	if err := sqlDB.QueryRowContext(
		ctx,
		"SELECT current_database()",
	).Scan(&databaseName); err != nil {
		t.Fatalf("get database name: %v", err)
	}

	if err := db.AutoMigrate(
		&infra.UserModel{},
		&infra.ScanModel{},
		&infra.IndicatorModel{},
		&infra.AuditLogModel{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return db
}

func createTestUser(
	t *testing.T,
	db *gorm.DB,
	email string,
	password string,
	role string,
) infra.UserModel {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}

	user := infra.UserModel{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Delete(&infra.UserModel{}, user.ID).Error; err != nil {
			t.Errorf("delete test user %d: %v", user.ID, err)
		}
	})

	return user
}

func newTestRouter(db *gorm.DB) *fiber.App {
	return app.NewRouter(app.Dependencies{
		DB:        db,
		JWTSecret: integrationJWTSecret,
	})
}

func performRequest(
	t *testing.T,
	router *fiber.App,
	method string,
	token string,
	path string,
	body string,
) (*http.Response, []byte) {
	t.Helper()

	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := router.Test(req)
	if err != nil {
		t.Fatalf("execute HTTP request: %v", err)
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return resp, data
}
