package services_scan

import (
	"context"
	"log/slog"

	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	ScanStatusPending    = "pending"
	ScanStatusProcessing = "processing"
	ScanStatusCompleted  = "completed"
	ScanStatusFailed     = "failed"
)
const ScanSubject = "phishing.scan.request"

type MessagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

type Scan struct {
	db        *gorm.DB
	redis     *redis.Client
	publisher MessagePublisher
	log       *slog.Logger
	subject   string
}

type Analysis struct {
	NormalizedURL string                 `json:"normalizedUrl"`
	Score         int                    `json:"score"`
	RiskLevel     string                 `json:"riskLevel"`
	Indicators    []infra.IndicatorModel `json:"indicators"`
}
