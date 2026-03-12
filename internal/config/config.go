package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	Port                   string
	SecretKey              string
	DBHost                 string
	DBPort                 string
	DBName                 string
	DBUser                 string
	DBPassword             string
	DBSSLMode              string
	DBTimeZone             string
	CookieSecure           bool
	SessionMaxAge          int
	GinMode                string
	TrustedProxies         []string
	UploadDir              string
	UploadURLBase          string
	HTTPReadTimeoutSec     int
	HTTPWriteTimeoutSec    int
	HTTPIdleTimeoutSec     int
	HTTPShutdownTimeoutSec int
}

func Load() (AppConfig, error) {
	cfg := AppConfig{
		Port:                   envOrDefault("PORT", "5001"),
		SecretKey:              envOrDefault("SECRET_KEY", "dev-secret-key-change-me"),
		DBHost:                 strings.TrimSpace(os.Getenv("DB_HOST")),
		DBPort:                 envOrDefault("DB_PORT", "5432"),
		DBName:                 envOrDefault("DB_NAME", "duty_log"),
		DBUser:                 envOrDefault("DB_USER", "postgres"),
		DBPassword:             strings.TrimSpace(os.Getenv("DB_PASSWORD")),
		DBSSLMode:              envOrDefault("DB_SSLMODE", "disable"),
		DBTimeZone:             envOrDefault("DB_TIMEZONE", "Asia/Shanghai"),
		CookieSecure:           envBool("COOKIE_SECURE", false),
		SessionMaxAge:          envInt("SESSION_MAX_AGE", 7*24*3600),
		GinMode:                normalizeGinMode(envOrDefault("GIN_MODE", "release")),
		TrustedProxies:         parseCSVEnv("TRUSTED_PROXIES", "127.0.0.1,::1"),
		UploadDir:              normalizeUploadDir(envOrDefault("UPLOAD_DIR", "./static/uploads")),
		UploadURLBase:          normalizeURLPath(envOrDefault("UPLOAD_URL_BASE", "/static/uploads")),
		HTTPReadTimeoutSec:     envInt("HTTP_READ_TIMEOUT_SEC", 15),
		HTTPWriteTimeoutSec:    envInt("HTTP_WRITE_TIMEOUT_SEC", 30),
		HTTPIdleTimeoutSec:     envInt("HTTP_IDLE_TIMEOUT_SEC", 60),
		HTTPShutdownTimeoutSec: envInt("HTTP_SHUTDOWN_TIMEOUT_SEC", 10),
	}

	var missing []string
	if cfg.DBHost == "" {
		missing = append(missing, "DB_HOST")
	}
	if cfg.DBName == "" {
		missing = append(missing, "DB_NAME")
	}
	if cfg.DBUser == "" {
		missing = append(missing, "DB_USER")
	}
	if cfg.DBPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if len(missing) > 0 {
		return AppConfig{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	if cfg.HTTPReadTimeoutSec <= 0 {
		cfg.HTTPReadTimeoutSec = 15
	}
	if cfg.HTTPWriteTimeoutSec <= 0 {
		cfg.HTTPWriteTimeoutSec = 30
	}
	if cfg.HTTPIdleTimeoutSec <= 0 {
		cfg.HTTPIdleTimeoutSec = 60
	}
	if cfg.HTTPShutdownTimeoutSec <= 0 {
		cfg.HTTPShutdownTimeoutSec = 10
	}
	return cfg, nil
}

func (c AppConfig) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		c.DBHost,
		c.DBUser,
		c.DBPassword,
		c.DBName,
		c.DBPort,
		c.DBSSLMode,
		c.DBTimeZone,
	)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCSVEnv(key, fallback string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		raw = fallback
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		result = append(result, v)
	}
	return result
}

func normalizeGinMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release":
		return "release"
	case "test":
		return "test"
	default:
		return "debug"
	}
}

func normalizeUploadDir(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return "./static/uploads"
	}
	return trimmed
}

func normalizeURLPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}
