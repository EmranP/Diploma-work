package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port,
	DBUrl,
	RedisAddr,
	NatsURL,
	JwtSecret,
	AdminEmail,
	AdminPassword,
	TestEmail,
	TestPassword string
}

func LoadConfig() *Config {
	err := godotenv.Load("./.env")
	if err != nil {
		log.Printf("%s", err.Error())
	}

	return &Config{
		Port:          env("APP_PORT", "8080"),
		DBUrl:         env("DB_URL", ""),
		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		NatsURL:       env("NATS_URL", "nats://localhost:4222"),
		JwtSecret:     env("JWT_SECRET", "super-jwt"),
		AdminEmail:    env("ADMIN_EMAIL", "admin@admin.com"),
		AdminPassword: env("ADMIN_PASSWORD", "qwert"),
		TestEmail:     env("TEST_EMAIL", "test@test.com"),
		TestPassword:  env("TEST_PASSWORD", "12345")}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
