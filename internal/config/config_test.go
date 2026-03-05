package config

import "testing"

func TestLoadSuccessAndNormalization(t *testing.T) {
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")
	t.Setenv("GIN_MODE", "invalid-mode")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.1 , 10.0.0.2 ")
	t.Setenv("HTTP_READ_TIMEOUT_SEC", "-1")
	t.Setenv("HTTP_WRITE_TIMEOUT_SEC", "abc")
	t.Setenv("HTTP_IDLE_TIMEOUT_SEC", "0")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT_SEC", "-3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected load success, got error: %v", err)
	}

	if cfg.DBName != "duty_log" {
		t.Fatalf("expected default DB_NAME duty_log, got %q", cfg.DBName)
	}
	if cfg.DBUser != "postgres" {
		t.Fatalf("expected default DB_USER postgres, got %q", cfg.DBUser)
	}
	if cfg.GinMode != "debug" {
		t.Fatalf("expected invalid gin mode to fallback to debug, got %q", cfg.GinMode)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.1" || cfg.TrustedProxies[1] != "10.0.0.2" {
		t.Fatalf("unexpected trusted proxies: %#v", cfg.TrustedProxies)
	}
	if cfg.HTTPReadTimeoutSec != 15 {
		t.Fatalf("expected read timeout fallback 15, got %d", cfg.HTTPReadTimeoutSec)
	}
	if cfg.HTTPWriteTimeoutSec != 30 {
		t.Fatalf("expected write timeout fallback 30, got %d", cfg.HTTPWriteTimeoutSec)
	}
	if cfg.HTTPIdleTimeoutSec != 60 {
		t.Fatalf("expected idle timeout fallback 60, got %d", cfg.HTTPIdleTimeoutSec)
	}
	if cfg.HTTPShutdownTimeoutSec != 10 {
		t.Fatalf("expected shutdown timeout fallback 10, got %d", cfg.HTTPShutdownTimeoutSec)
	}
}

func TestLoadMissingRequiredEnv(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected missing required env error")
	}
}
