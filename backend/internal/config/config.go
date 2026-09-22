// Package config contains application-wide configuration primitives.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the root configuration container.
type Config struct {
	Environment     string
	Address         string
	DatabaseURL     string
	JWTSecret       string
	FrontendOrigin  string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieSecure    bool
	CookieSameSite  string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Environment:     envOrDefault("APP_ENV", "development"),
		Address:         envOrDefault("SERVER_ADDRESS", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:       strings.TrimSpace(os.Getenv("JWT_SECRET")),
		FrontendOrigin:  strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN")),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		CookieSameSite:  envOrDefault("COOKIE_SAME_SITE", "lax"),
	}

	var err error
	if cfg.CookieSecure, err = strconv.ParseBool(envOrDefault("COOKIE_SECURE", "false")); err != nil {
		return Config{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" || cfg.FrontendOrigin == "" {
		return Config{}, fmt.Errorf("DATABASE_URL, JWT_SECRET and FRONTEND_ORIGIN must be set")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	if cfg.CookieSameSite != "lax" && cfg.CookieSameSite != "strict" && cfg.CookieSameSite != "none" {
		return Config{}, fmt.Errorf("COOKIE_SAME_SITE must be lax, strict or none")
	}
	if cfg.CookieSameSite == "none" && !cfg.CookieSecure {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true when COOKIE_SAME_SITE is none")
	}
	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
