package middleware

import (
	"net/http/httptest"
	"testing"

	"duty-log-system/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestParseSessionUserID(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want uint
		ok   bool
	}{
		{name: "int", in: 12, want: 12, ok: true},
		{name: "int64", in: int64(34), want: 34, ok: true},
		{name: "uint", in: uint(56), want: 56, ok: true},
		{name: "float64", in: float64(78), want: 78, ok: true},
		{name: "string", in: "90", want: 90, ok: true},
		{name: "string with spaces", in: " 91 ", want: 91, ok: true},
		{name: "zero string", in: "0", want: 0, ok: false},
		{name: "invalid string", in: "abc", want: 0, ok: false},
		{name: "negative int", in: -1, want: 0, ok: false},
		{name: "nil", in: nil, want: 0, ok: false},
	}

	for _, tc := range cases {
		got, ok := parseSessionUserID(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%s: got=(%d,%v) want=(%d,%v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCurrentUserFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	if user, ok := currentUserFromContext(ctx); ok || user != nil {
		t.Fatalf("expected empty context to have no user")
	}

	ctx.Set(currentUserContextKey, "invalid")
	if user, ok := currentUserFromContext(ctx); ok || user != nil {
		t.Fatalf("expected invalid context value to be ignored")
	}

	expected := &models.User{ID: 7, Username: "tester", IsActive: true}
	ctx.Set(currentUserContextKey, expected)
	got, ok := currentUserFromContext(ctx)
	if !ok || got == nil {
		t.Fatalf("expected cached user from context")
	}
	if got.ID != expected.ID || got.Username != expected.Username {
		t.Fatalf("unexpected cached user: %+v", got)
	}
}

func TestCurrentUserUsesContextCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	expected := &models.User{ID: 11, Username: "cached", IsActive: true}
	ctx.Set(currentUserContextKey, expected)

	user, err := CurrentUser(ctx, nil)
	if err != nil {
		t.Fatalf("CurrentUser should use context cache: %v", err)
	}
	if user == nil || user.ID != expected.ID {
		t.Fatalf("unexpected user from context cache: %+v", user)
	}
}

func TestCurrentUserRejectsInactiveContextUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(currentUserContextKey, &models.User{ID: 12, Username: "inactive", IsActive: false})

	user, err := CurrentUser(ctx, nil)
	if err == nil {
		t.Fatalf("expected error for inactive context user, got user=%+v", user)
	}
}

func TestSetLoginSessionClearsTransient2FAState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore([]byte("test-secret-key"))
	r.Use(sessions.Sessions("test-session", store))
	r.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set(SessionPendingOTPSecret, "secret")
		session.Set(SessionPendingOTPAUTHURL, "otpauth://totp/test")
		session.Set(SessionPendingOTPUsername, "tester")
		session.Set(Session2FAFailedAttempts, 3)
		session.Set(Session2FABlockedUntil, 1234567890)
		if err := session.Save(); err != nil {
			t.Fatalf("seed session failed: %v", err)
		}

		if err := SetLoginSession(c, 42, false); err != nil {
			t.Fatalf("SetLoginSession failed: %v", err)
		}

		session = sessions.Default(c)
		if got := session.Get(SessionPendingOTPSecret); got != nil {
			t.Fatalf("pending otp secret should be cleared, got=%v", got)
		}
		if got := session.Get(SessionPendingOTPAUTHURL); got != nil {
			t.Fatalf("pending otp auth url should be cleared, got=%v", got)
		}
		if got := session.Get(SessionPendingOTPUsername); got != nil {
			t.Fatalf("pending otp username should be cleared, got=%v", got)
		}
		if got := session.Get(Session2FAFailedAttempts); got != nil {
			t.Fatalf("failed attempts should be cleared, got=%v", got)
		}
		if got := session.Get(Session2FABlockedUntil); got != nil {
			t.Fatalf("blocked-until should be cleared, got=%v", got)
		}

		if got := session.Get(SessionUserIDKey); got == nil {
			t.Fatalf("user_id should be set")
		}
		if got := session.Get(Session2FAVerifiedKey); got != false {
			t.Fatalf("two_factor_verified should be false, got=%v", got)
		}
		c.Status(204)
	})

	req := httptest.NewRequest("GET", "/login", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != 204 {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}
