// Package config loads vitrine's configuration from environment
// variables, all prefixed with VITRINE_.
package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	minSecretLen = 32

	defaultAddr       = ":8080"
	defaultDBPath     = "./vitrine.db"
	defaultTheme      = "plain"
	defaultLogLevel   = "info"
	defaultThemesDir  = "/themes"
)

// Config holds the fully validated runtime configuration.
type Config struct {
	Addr          string
	DBPath        string
	WebhookSecret string
	BaseURL       string
	Theme         string
	LogLevel      string
	Dev           bool
	DemoPayload   string
	PoweredBy     string
	ThemesDir     string // directory containing theme subdirectories (optional, defaults to "")
}

// Load reads and validates configuration from the environment. It never
// calls os.Exit; callers decide how to react to a returned error.
func Load() (Config, error) {
	cfg := Config{
		Addr:          getEnv("VITRINE_ADDR", defaultAddr),
		DBPath:        getEnv("VITRINE_DB_PATH", defaultDBPath),
		WebhookSecret: os.Getenv("VITRINE_WEBHOOK_SECRET"),
		BaseURL:       os.Getenv("VITRINE_BASE_URL"),
		Theme:         getEnv("VITRINE_THEME", defaultTheme),
		LogLevel:      getEnv("VITRINE_LOG_LEVEL", defaultLogLevel),
		DemoPayload:   os.Getenv("VITRINE_DEMO_PAYLOAD"),
		PoweredBy:     getEnv("VITRINE_POWERED_BY", "MyCompany"),
		ThemesDir:     os.Getenv("VITRINE_THEMES_DIR"),
	}

	if cfg.WebhookSecret == "" {
		return Config{}, fmt.Errorf("VITRINE_WEBHOOK_SECRET is required")
	}
	if len(cfg.WebhookSecret) < minSecretLen {
		return Config{}, fmt.Errorf("VITRINE_WEBHOOK_SECRET must be at least %d characters, got %d", minSecretLen, len(cfg.WebhookSecret))
	}
	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("VITRINE_BASE_URL is required")
	}

	dev, err := getEnvBool("VITRINE_DEV", false)
	if err != nil {
		return Config{}, err
	}
	cfg.Dev = dev

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean, got %q", key, v)
	}
	return b, nil
}
