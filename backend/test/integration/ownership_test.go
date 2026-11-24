package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	appRouter "github.com/patdyyew-emran/phishing-detector/internal/app"
	authpkg "github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"

	"gorm.io/gorm"
)

func newOwnershipRouter(db *gorm.DB) *fiber.App {
	scanService := services_scan.NewServices(
		db,
		nil,
		nil,
		slog.Default(),
	)

	return appRouter.NewRouter(appRouter.Dependencies{
		DB:          db,
		ScanService: scanService,
		JWTSecret:   integrationJWTSecret,
	})
}

func createOwnershipToken(
	t *testing.T,
	user infra.UserModel,
) string {
	t.Helper()

	now := time.Now()

	claims := authpkg.AccessClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authpkg.TokenIssuer,
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			Audience:  jwt.ClaimStrings{authpkg.TokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signed, err := token.SignedString(
		[]byte(integrationJWTSecret),
	)
	if err != nil {
		t.Fatalf("sign integration token: %v", err)
	}

	return signed
}

func ownershipRequest(
	t *testing.T,
	router *fiber.App,
	method string,
	path string,
	token string,
) (int, []byte) {
	t.Helper()

	req := httptest.NewRequest(
		method,
		path,
		nil,
	)

	if token != "" {
		req.Header.Set(
			"Authorization",
			"Bearer "+token,
		)
	}

	resp, err := router.Test(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	return resp.StatusCode, body
}

func createOwnershipUser(
	t *testing.T,
	db *gorm.DB,
	name string,
	role string,
) infra.UserModel {
	t.Helper()

	email := fmt.Sprintf(
		"%s-%d@example.test",
		name,
		time.Now().UnixNano(),
	)

	return createTestUser(
		t,
		db,
		email,
		"ownership-test-password",
		role,
	)
}

func createOwnershipScan(
	t *testing.T,
	db *gorm.DB,
	userID uint,
	name string,
	riskLevel string,
) infra.ScanModel {
	t.Helper()

	scan := infra.ScanModel{
		UserID:        userID,
		URL:           fmt.Sprintf("https://example.test/%s", name),
		NormalizedURL: fmt.Sprintf("https://example.test/%s", name),
		Status:        "completed",
		RiskLevel:     riskLevel,
	}

	if err := db.Create(&scan).Error; err != nil {
		t.Fatalf("create test scan: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Delete(
			&infra.ScanModel{},
			scan.ID,
		).Error; err != nil {
			t.Errorf("delete test scan %d: %v", scan.ID, err)
		}
	})

	return scan
}

type ownershipScanListResponse struct {
	Items []infra.ScanModel `json:"items"`
	Total int64             `json:"total"`
	Limit int               `json:"limit"`
}

func TestScanOwnershipIsolation(t *testing.T) {
	db := openTestDB(t)
	router := newOwnershipRouter(db)

	userA := createOwnershipUser(
		t,
		db,
		"owner-a",
		authpkg.RoleUser,
	)

	userB := createOwnershipUser(
		t,
		db,
		"owner-b",
		authpkg.RoleUser,
	)

	admin := createOwnershipUser(
		t,
		db,
		"owner-admin",
		authpkg.RoleAdmin,
	)

	tokenA := createOwnershipToken(t, userA)
	tokenB := createOwnershipToken(t, userB)
	adminToken := createOwnershipToken(t, admin)

	scanA1 := createOwnershipScan(
		t,
		db,
		userA.ID,
		"owner-a-1",
		"low",
	)

	scanA2 := createOwnershipScan(
		t,
		db,
		userA.ID,
		"owner-a-2",
		"high",
	)

	scanB1 := createOwnershipScan(
		t,
		db,
		userB.ID,
		"owner-b-1",
		"low",
	)

	t.Run("unauthenticated user cannot list scans", func(t *testing.T) {
		status, _ := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/scans/",
			"",
		)

		if status != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", status)
		}
	})

	t.Run("user A sees only own scans", func(t *testing.T) {
		status, body := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/scans/?limit=100",
			tokenA,
		)

		if status != http.StatusOK {
			t.Fatalf(
				"expected 200, got %d: %s",
				status,
				body,
			)
		}

		var result ownershipScanListResponse

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if result.Total != 2 {
			t.Fatalf(
				"expected 2 scans, got %d",
				result.Total,
			)
		}

		if len(result.Items) != 2 {
			t.Fatalf(
				"expected 2 items, got %d",
				len(result.Items),
			)
		}

		for _, scan := range result.Items {
			if scan.UserID != userA.ID {
				t.Fatalf(
					"user A received scan belonging to user %d",
					scan.UserID,
				)
			}
		}
	})

	t.Run("user B sees only own scans", func(t *testing.T) {
		status, body := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/scans/?limit=100",
			tokenB,
		)

		if status != http.StatusOK {
			t.Fatalf(
				"expected 200, got %d: %s",
				status,
				body,
			)
		}

		var result ownershipScanListResponse

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if result.Total != 1 || len(result.Items) != 1 {
			t.Fatalf(
				"expected 1 scan, got total=%d items=%d",
				result.Total,
				len(result.Items),
			)
		}

		if result.Items[0].ID != scanB1.ID {
			t.Fatalf(
				"expected scan %d, got %d",
				scanB1.ID,
				result.Items[0].ID,
			)
		}

		if result.Items[0].UserID != userB.ID {
			t.Fatalf("user B received another user's scan")
		}
	})

	t.Run("admin sees scans from all users", func(t *testing.T) {
		status, body := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/scans/?limit=100",
			adminToken,
		)

		if status != http.StatusOK {
			t.Fatalf(
				"expected 200, got %d: %s",
				status,
				body,
			)
		}

		var result ownershipScanListResponse

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		found := make(map[uint]bool)

		for _, scan := range result.Items {
			found[scan.ID] = true
		}

		expectedIDs := []uint{
			scanA1.ID,
			scanA2.ID,
			scanB1.ID,
		}

		for _, id := range expectedIDs {
			if !found[id] {
				t.Errorf(
					"admin cannot see scan %d",
					id,
				)
			}
		}
	})
}

func TestGetScanOwnership(t *testing.T) {
	db := openTestDB(t)
	router := newOwnershipRouter(db)

	userA := createOwnershipUser(
		t,
		db,
		"get-owner-a",
		authpkg.RoleUser,
	)

	userB := createOwnershipUser(
		t,
		db,
		"get-owner-b",
		authpkg.RoleUser,
	)

	admin := createOwnershipUser(
		t,
		db,
		"get-admin",
		authpkg.RoleAdmin,
	)

	scan := createOwnershipScan(
		t,
		db,
		userA.ID,
		"get-owner-a-scan",
		"high",
	)

	tokenA := createOwnershipToken(t, userA)
	tokenB := createOwnershipToken(t, userB)
	adminToken := createOwnershipToken(t, admin)

	scanPath := fmt.Sprintf("/api/scans/%d", scan.ID)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "owner can get own scan",
			token:          tokenA,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "another user cannot get scan",
			token:          tokenB,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "admin can get any scan",
			token:          adminToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthenticated request",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := ownershipRequest(
				t,
				router,
				http.MethodGet,
				scanPath,
				tt.token,
			)

			if status != tt.expectedStatus {
				t.Fatalf(
					"expected %d, got %d: %s",
					tt.expectedStatus,
					status,
					body,
				)
			}

			if status == http.StatusOK {
				var result infra.ScanModel

				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("decode scan: %v", err)
				}

				if result.ID != scan.ID {
					t.Fatalf(
						"expected scan ID %d, got %d",
						scan.ID,
						result.ID,
					)
				}
			}
		})
	}
}

func TestStatsRBAC(t *testing.T) {
	db := openTestDB(t)
	router := newOwnershipRouter(db)

	user := createOwnershipUser(
		t,
		db,
		"stats-user",
		authpkg.RoleUser,
	)

	admin := createOwnershipUser(
		t,
		db,
		"stats-admin",
		authpkg.RoleAdmin,
	)

	userToken := createOwnershipToken(t, user)
	adminToken := createOwnershipToken(t, admin)

	var totalBefore int64
	if err := db.Model(&infra.ScanModel{}).
		Count(&totalBefore).Error; err != nil {
		t.Fatal(err)
	}

	var highRiskBefore int64
	if err := db.Model(&infra.ScanModel{}).
		Where(
			"risk_level IN ?",
			[]string{"high", "critical"},
		).
		Count(&highRiskBefore).Error; err != nil {
		t.Fatal(err)
	}

	createOwnershipScan(t, db, user.ID, "stats-low", "low")
	createOwnershipScan(t, db, user.ID, "stats-high", "high")
	createOwnershipScan(t, db, user.ID, "stats-critical", "critical")

	t.Run("regular user cannot access stats", func(t *testing.T) {
		status, body := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/stats/",
			userToken,
		)

		if status != http.StatusForbidden {
			t.Fatalf(
				"expected 403, got %d: %s",
				status,
				body,
			)
		}
	})

	t.Run("admin can access stats", func(t *testing.T) {
		status, body := ownershipRequest(
			t,
			router,
			http.MethodGet,
			"/api/stats/",
			adminToken,
		)

		if status != http.StatusOK {
			t.Fatalf(
				"expected 200, got %d: %s",
				status,
				body,
			)
		}

		var result struct {
			TotalScans    int64 `json:"totalScans"`
			HighRiskScans int64 `json:"highRiskScans"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode stats response: %v", err)
		}

		expectedTotal := totalBefore + 3
		expectedHighRisk := highRiskBefore + 2

		if result.TotalScans != expectedTotal {
			t.Errorf(
				"expected total scans %d, got %d",
				expectedTotal,
				result.TotalScans,
			)
		}

		if result.HighRiskScans != expectedHighRisk {
			t.Errorf(
				"expected high-risk scans %d, got %d",
				expectedHighRisk,
				result.HighRiskScans,
			)
		}
	})
}
