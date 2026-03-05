package middleware

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type loginRateLimiter struct {
	mu          sync.Mutex
	entries     map[string]loginAttempt
	maxAttempts int
	window      time.Duration
	block       time.Duration
	lastCleanup time.Time
}

type loginAttempt struct {
	firstFail    time.Time
	fails        int
	blockedUntil time.Time
}

var defaultLoginRateLimiter = &loginRateLimiter{
	entries:     make(map[string]loginAttempt),
	maxAttempts: 5,
	window:      10 * time.Minute,
	block:       15 * time.Minute,
}

func ConfigureLoginRateLimiter(maxAttempts int, window, block time.Duration) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	if block <= 0 {
		block = 15 * time.Minute
	}

	defaultLoginRateLimiter.mu.Lock()
	defaultLoginRateLimiter.maxAttempts = maxAttempts
	defaultLoginRateLimiter.window = window
	defaultLoginRateLimiter.block = block
	defaultLoginRateLimiter.mu.Unlock()
}

func LoginRateKey(c *gin.Context, username string) string {
	normalizedUsername := strings.ToLower(strings.TrimSpace(username))
	if normalizedUsername == "" {
		normalizedUsername = "_"
	}
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}
	return fmt.Sprintf("%s|%s", ip, normalizedUsername)
}

func IsLoginAllowed(key string) (bool, time.Duration) {
	now := time.Now()

	defaultLoginRateLimiter.mu.Lock()
	defer defaultLoginRateLimiter.mu.Unlock()
	defaultLoginRateLimiter.cleanup(now)

	entry, ok := defaultLoginRateLimiter.entries[key]
	if !ok {
		return true, 0
	}
	if entry.blockedUntil.After(now) {
		return false, entry.blockedUntil.Sub(now)
	}
	return true, 0
}

func RecordLoginFailure(key string) {
	now := time.Now()

	defaultLoginRateLimiter.mu.Lock()
	defer defaultLoginRateLimiter.mu.Unlock()
	defaultLoginRateLimiter.cleanup(now)

	entry, ok := defaultLoginRateLimiter.entries[key]
	if !ok || entry.firstFail.IsZero() || now.Sub(entry.firstFail) > defaultLoginRateLimiter.window {
		entry = loginAttempt{firstFail: now, fails: 0}
	}

	entry.fails++
	if entry.fails >= defaultLoginRateLimiter.maxAttempts {
		entry.blockedUntil = now.Add(defaultLoginRateLimiter.block)
		entry.firstFail = time.Time{}
		entry.fails = 0
	}
	defaultLoginRateLimiter.entries[key] = entry
}

func RecordLoginSuccess(key string) {
	defaultLoginRateLimiter.mu.Lock()
	delete(defaultLoginRateLimiter.entries, key)
	defaultLoginRateLimiter.mu.Unlock()
}

func (l *loginRateLimiter) cleanup(now time.Time) {
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < 2*time.Minute {
		return
	}
	for key, entry := range l.entries {
		expiredWindow := !entry.firstFail.IsZero() && now.Sub(entry.firstFail) > l.window
		expiredBlock := !entry.blockedUntil.IsZero() && now.After(entry.blockedUntil)
		if expiredWindow && (entry.blockedUntil.IsZero() || expiredBlock) {
			delete(l.entries, key)
			continue
		}
		if entry.firstFail.IsZero() && expiredBlock {
			delete(l.entries, key)
		}
	}
	l.lastCleanup = now
}

func ParsePositiveInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
