package unit_test

import (
	"testing"

	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
)

func TestInspectURLDetectsHighRiskURL(t *testing.T) {
	result, err := services_scan.InspectURL("http://xn--secure-login-bank---test.example.com/login?account=1")
	if err != nil {
		t.Fatal(err)
	}
	if result.RiskLevel != "high" && result.RiskLevel != "critical" {
		t.Fatalf("expected high risk, got %s with score %d", result.RiskLevel, result.Score)
	}
	if len(result.Indicators) < 3 {
		t.Fatalf("expected several indicators, got %d", len(result.Indicators))
	}
}

func TestInspectURLAddsHTTPSWhenSchemeIsMissing(t *testing.T) {
	result, err := services_scan.InspectURL("example.com/docs")
	if err != nil {
		t.Fatal(err)
	}
	if result.NormalizedURL != "https://example.com/docs" {
		t.Fatalf("unexpected normalized URL: %s", result.NormalizedURL)
	}
}
