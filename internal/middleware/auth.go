package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"duty-log-system/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	SessionUserIDKey          = "user_id"
	Session2FAVerifiedKey     = "two_factor_verified"
	SessionPendingOTPSecret   = "pending_otp_secret"
	SessionPendingOTPAUTHURL  = "pending_otp_auth_url"
	SessionPendingOTPUsername = "pending_otp_username"
	Session2FAFailedAttempts  = "two_factor_failed_attempts"
	Session2FABlockedUntil    = "two_factor_blocked_until_unix"
	currentUserContextKey     = "current_user"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentUserID(c); !ok {
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func LoadCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := CurrentUserID(c)
		if !ok {
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}

		var user models.User
		if err := db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
			_ = ClearSession(c)
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}

		c.Set(currentUserContextKey, &user)
		c.Next()
	}
}

func AdminRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := CurrentUser(c, db)
		if err != nil {
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}
		if !user.IsAdmin {
			c.HTML(http.StatusForbidden, "coming_soon.html", gin.H{
				"Title":   "无权限",
				"Message": "该页面仅管理员可访问。",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func Require2FA(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := CurrentUser(c, db)
		if err != nil {
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}
		if user.OTPSecret == "" {
			c.Next()
			return
		}
		if Is2FAVerified(c) {
			c.Next()
			return
		}
		c.Redirect(http.StatusFound, "/auth/verify-2fa")
		c.Abort()
	}
}

func CurrentUserID(c *gin.Context) (uint, bool) {
	session := sessions.Default(c)
	return parseSessionUserID(session.Get(SessionUserIDKey))
}

func parseSessionUserID(raw any) (uint, bool) {
	switch v := raw.(type) {
	case int:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	case int64:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	case uint:
		if v == 0 {
			return 0, false
		}
		return v, true
	case float64:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil || parsed == 0 {
			return 0, false
		}
		return uint(parsed), true
	default:
		return 0, false
	}
}

func CurrentUser(c *gin.Context, db *gorm.DB) (*models.User, error) {
	if user, ok := currentUserFromContext(c); ok {
		if !user.IsActive {
			return nil, errors.New("inactive user")
		}
		return user, nil
	}

	if db == nil {
		return nil, errors.New("db is required")
	}

	userID, ok := CurrentUserID(c)
	if !ok {
		return nil, errors.New("no active session")
	}
	var user models.User
	if err := db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func currentUserFromContext(c *gin.Context) (*models.User, bool) {
	if c == nil {
		return nil, false
	}
	raw, exists := c.Get(currentUserContextKey)
	if !exists {
		return nil, false
	}
	user, ok := raw.(*models.User)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

func Is2FAVerified(c *gin.Context) bool {
	session := sessions.Default(c)
	raw := session.Get(Session2FAVerifiedKey)
	verified, ok := raw.(bool)
	return ok && verified
}

func SetLoginSession(c *gin.Context, userID uint, verified bool) error {
	session := sessions.Default(c)
	clearTransientAuthSessionState(session)
	session.Set(SessionUserIDKey, int(userID))
	session.Set(Session2FAVerifiedKey, verified)
	return session.Save()
}

func Set2FAVerified(c *gin.Context, verified bool) error {
	session := sessions.Default(c)
	session.Set(Session2FAVerifiedKey, verified)
	return session.Save()
}

func ClearSession(c *gin.Context) error {
	session := sessions.Default(c)
	session.Clear()
	return session.Save()
}

func clearTransientAuthSessionState(session sessions.Session) {
	if session == nil {
		return
	}
	session.Delete(SessionPendingOTPSecret)
	session.Delete(SessionPendingOTPAUTHURL)
	session.Delete(SessionPendingOTPUsername)
	session.Delete(Session2FAFailedAttempts)
	session.Delete(Session2FABlockedUntil)
}
