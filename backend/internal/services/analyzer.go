package services_scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"github.com/redis/go-redis/v9"
)

func InspectURL(rawURL string) (Analysis, error) {
	value := strings.TrimSpace(rawURL)
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Analysis{}, fmt.Errorf("URL formady nädogry")
	}
	host := strings.ToLower(parsed.Hostname())
	result := Analysis{NormalizedURL: parsed.String()}
	add := func(code, description string, weight int) {
		result.Indicators = append(result.Indicators, infra.IndicatorModel{Code: code, Description: description, Weight: weight})
		result.Score += weight
	}
	if parsed.Scheme == "http" {
		add("HTTP_ONLY", "HTTPS ulanmaýan URL", 20)
	}
	if net.ParseIP(host) != nil {
		add("IP_HOST", "Domeniň ýerine IP salgy ulanylýar", 35)
	}
	if strings.Contains(host, "xn--") {
		add("PUNYCODE", "Punycode domeni ýüze çykaryldy", 25)
	}
	if strings.Count(host, "-") >= 3 {
		add("MANY_HYPHENS", "Domen adynda köp defis bar", 12)
	}
	if strings.Count(host, ".") >= 4 {
		add("MANY_SUBDOMAINS", "Domen adynda köp subdomen bar", 12)
	}
	if len(value) > 100 {
		add("LONG_URL", "URL adatdan daşary uzyn", 10)
	}
	if strings.Contains(value, "@") {
		add("AT_SYMBOL", "URL-de @ belgisi bar", 25)
	}
	suspiciousWords := []string{"login", "verify", "password", "secure", "account", "bank", "bonus", "confirm", "update"}
	for _, word := range suspiciousWords {
		if strings.Contains(strings.ToLower(value), word) {
			add("SUSPICIOUS_WORD", "Şübheli söz: "+word, 7)
		}
	}
	if result.Score > 100 {
		result.Score = 100
	}
	switch {
	case result.Score >= 75:
		result.RiskLevel = "critical"
	case result.Score >= 45:
		result.RiskLevel = "high"
	case result.Score >= 20:
		result.RiskLevel = "medium"
	default:
		result.RiskLevel = "low"
	}
	return result, nil
}

func (s *Scan) Analyze(
	ctx context.Context,
	rawURL string,
) (Analysis, error) {
	if ctx == nil {
		return Analysis{}, errors.New("analysis context is nil")
	}

	if err := ctx.Err(); err != nil {
		return Analysis{}, err
	}

	key := "scan:" + strings.TrimSpace(rawURL)

	if s.redis != nil {
		cached, err := s.redis.Get(ctx, key).Bytes()

		if ctxErr := ctx.Err(); ctxErr != nil {
			return Analysis{}, ctxErr
		}

		switch {
		case err == nil:
			var result Analysis

			if unmarshalErr := json.Unmarshal(cached, &result); unmarshalErr == nil {
				return result, nil
			}

			s.workerLogger().Warn(
				"invalid cached scan analysis; recalculating",
			)

		case errors.Is(err, redis.Nil):
			// Cache miss. Continue with URL analysis.

		case errors.Is(err, context.Canceled),
			errors.Is(err, context.DeadlineExceeded):

			return Analysis{}, err

		default:
			// Redis is an optional cache.
			// Continue without cached data.
			s.workerLogger().Warn(
				"Redis cache lookup failed; continuing without cache",
				"error", err,
			)
		}
	}

	result, err := InspectURL(rawURL)
	if err != nil {
		return Analysis{}, err
	}

	if err := ctx.Err(); err != nil {
		return Analysis{}, err
	}

	if s.redis != nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			s.workerLogger().Warn(
				"failed to serialize scan analysis cache",
				"error", err,
			)
		} else {
			err = s.redis.Set(
				ctx,
				key,
				encoded,
				10*time.Minute,
			).Err()

			if ctxErr := ctx.Err(); ctxErr != nil {
				return Analysis{}, ctxErr
			}

			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return Analysis{}, err
			}

			if err != nil {
				s.workerLogger().Warn(
					"Redis cache write failed",
					"error", err,
				)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return Analysis{}, err
	}

	return result, nil
}
