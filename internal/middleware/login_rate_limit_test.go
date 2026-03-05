package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLoginRateKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	c.Request = req

	key := LoginRateKey(c, "  AdminUser  ")
	if !strings.Contains(key, "203.0.113.10") {
		t.Fatalf("expected ip in key, got %q", key)
	}
	if !strings.Contains(key, "adminuser") {
		t.Fatalf("expected normalized username in key, got %q", key)
	}
}

func TestRateLimitBlocksAfterThreshold(t *testing.T) {
	ConfigureLoginRateLimiter(2, 10*time.Second, 300*time.Millisecond)
	key := "test-block-threshold"

	allowed, _ := IsLoginAllowed(key)
	if !allowed {
		t.Fatalf("expected first attempt to be allowed")
	}

	RecordLoginFailure(key)
	RecordLoginFailure(key)

	allowed, retryAfter := IsLoginAllowed(key)
	if allowed {
		t.Fatalf("expected key to be blocked after threshold")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %v", retryAfter)
	}

	time.Sleep(350 * time.Millisecond)
	allowed, _ = IsLoginAllowed(key)
	if !allowed {
		t.Fatalf("expected key to be allowed after block expiration")
	}
}

func TestRecordLoginSuccessClearsState(t *testing.T) {
	ConfigureLoginRateLimiter(1, 10*time.Second, 5*time.Second)
	key := "test-success-clear"

	RecordLoginFailure(key)
	allowed, _ := IsLoginAllowed(key)
	if allowed {
		t.Fatalf("expected blocked before success clear")
	}

	RecordLoginSuccess(key)
	allowed, _ = IsLoginAllowed(key)
	if !allowed {
		t.Fatalf("expected allowed after success clear")
	}
}

func TestParsePositiveInt(t *testing.T) {
	if got := ParsePositiveInt("", 7); got != 7 {
		t.Fatalf("expected fallback for empty, got %d", got)
	}
	if got := ParsePositiveInt("abc", 7); got != 7 {
		t.Fatalf("expected fallback for invalid, got %d", got)
	}
	if got := ParsePositiveInt("-3", 7); got != 7 {
		t.Fatalf("expected fallback for negative, got %d", got)
	}
	if got := ParsePositiveInt("12", 7); got != 12 {
		t.Fatalf("expected parsed value 12, got %d", got)
	}
}
