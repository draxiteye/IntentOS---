package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string
	DemoMode   bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:       getEnv("PORT", "8080"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "intentos"),
		DBPassword: getEnv("DB_PASSWORD", "secret"),
		DBName:     getEnv("DB_NAME", "intentos"),
		JWTSecret:  getEnv("JWT_SECRET", "supersecret"),
	}
	// Demo mode if no AI key env var (e.g., OPENAI_API_KEY)
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		cfg.DemoMode = true
	}
	return cfg, nil
}

func (c *Config) DatabaseURL() string {
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	// Postgres URL format
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
