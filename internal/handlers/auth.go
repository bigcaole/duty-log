package handlers

import (
	"fmt"
	htemplate "html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"
	"duty-log-system/pkg/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

func registerAuthRoutes(router *gin.Engine, app *AppContext) {
	group := router.Group("/auth")
	authRequired := middleware.LoadCurrentUser(app.DB)
	{
		group.GET("/login", app.showLoginPage)
		group.POST("/login", app.login)
		group.GET("/logout", app.logout)

		group.GET("/setup-2fa", authRequired, app.showSetup2FA)
		group.POST("/setup-2fa", authRequired, app.setup2FA)
		group.GET("/verify-2fa", authRequired, app.showVerify2FA)
		group.POST("/verify-2fa", authRequired, app.verify2FA)
	}
}

func (a *AppContext) showLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/login.html", gin.H{
		"Title": "登录",
		"Error": c.Query("error"),
	})
}

func (a *AppContext) login(c *gin.Context) {
	a.applyLoginRateLimitConfig()

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	attemptKey := middleware.LoginRateKey(c, username)

	if allowed, retryAfter := middleware.IsLoginAllowed(attemptKey); !allowed {
		retrySeconds := int(retryAfter.Seconds()) + 1
		if retrySeconds < 1 {
			retrySeconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(retrySeconds))
		c.HTML(http.StatusTooManyRequests, "auth/login.html", gin.H{
			"Title": "登录",
			"Error": fmt.Sprintf("登录失败次数过多，请在 %d 秒后重试。", retrySeconds),
		})
		return
	}

	if username == "" || password == "" {
		middleware.RecordLoginFailure(attemptKey)
		c.HTML(http.StatusBadRequest, "auth/login.html", gin.H{
			"Title": "登录",
			"Error": "用户名和密码不能为空。",
		})
		return
	}

	var user models.User
	if err := a.DB.Where("username = ? AND is_active = ?", username, true).First(&user).Error; err != nil {
		middleware.RecordLoginFailure(attemptKey)
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"Title": "登录",
			"Error": "用户名或密码错误。",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		middleware.RecordLoginFailure(attemptKey)
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"Title": "登录",
			"Error": "用户名或密码错误。",
		})
		return
	}

	middleware.RecordLoginSuccess(attemptKey)

	verified := user.OTPSecret == ""
	if err := middleware.SetLoginSession(c, user.ID, verified); err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"Title": "登录",
			"Error": "会话创建失败，请稍后重试。",
		})
		return
	}

	if verified {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.Redirect(http.StatusFound, "/auth/verify-2fa")
}

func (a *AppContext) applyLoginRateLimitConfig() {
	maxAttempts := middleware.ParsePositiveInt(a.ConfigCenter.Get("LOGIN_MAX_ATTEMPTS", "5"), 5)
	windowSeconds := middleware.ParsePositiveInt(a.ConfigCenter.Get("LOGIN_WINDOW_SECONDS", "600"), 600)
	blockSeconds := middleware.ParsePositiveInt(a.ConfigCenter.Get("LOGIN_BLOCK_SECONDS", "900"), 900)

	middleware.ConfigureLoginRateLimiter(
		maxAttempts,
		time.Duration(windowSeconds)*time.Second,
		time.Duration(blockSeconds)*time.Second,
	)
}

func (a *AppContext) logout(c *gin.Context) {
	_ = middleware.ClearSession(c)
	c.Redirect(http.StatusFound, "/auth/login")
}

func (a *AppContext) showSetup2FA(c *gin.Context) {
	user, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	if strings.TrimSpace(user.OTPSecret) != "" {
		c.Redirect(http.StatusFound, "/auth/verify-2fa")
		return
	}

	issuer := a.ConfigCenter.Get("TOTP_ISSUER", "Duty-Log-System")
	account := user.Username
	if user.Email != "" {
		account = user.Email
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/setup_2fa.html", gin.H{
			"Title": "设置 2FA",
			"Error": "生成 TOTP 密钥失败。",
		})
		return
	}

	session := sessions.Default(c)
	session.Set(middleware.SessionPendingOTPSecret, key.Secret())
	session.Set(middleware.SessionPendingOTPAUTHURL, key.URL())
	session.Set(middleware.SessionPendingOTPUsername, account)
	_ = session.Save()

	qrCodeDataURL, _ := utils.GenerateOTPQRCodeDataURL(key.URL(), 240)

	c.HTML(http.StatusOK, "auth/setup_2fa.html", gin.H{
		"Title":         "设置 2FA",
		"Secret":        key.Secret(),
		"OtpAuthURL":    key.URL(),
		"AccountName":   account,
		"QRCodeDataURL": safeTemplateDataImageURL(qrCodeDataURL),
	})
}

func (a *AppContext) setup2FA(c *gin.Context) {
	user, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if user.OTPSecret != "" {
		c.Redirect(http.StatusFound, "/auth/verify-2fa")
		return
	}

	code := strings.TrimSpace(c.PostForm("code"))
	session := sessions.Default(c)
	rawSecret := session.Get(middleware.SessionPendingOTPSecret)
	secret, ok := rawSecret.(string)
	if !ok || strings.TrimSpace(secret) == "" {
		c.HTML(http.StatusBadRequest, "auth/setup_2fa.html", gin.H{
			"Title": "设置 2FA",
			"Error": "2FA 初始化状态失效，请重新加载页面。",
		})
		return
	}

	if !totp.Validate(code, secret) {
		otpAuthURL := sessionStringValue(session, middleware.SessionPendingOTPAUTHURL)
		accountName := sessionStringValue(session, middleware.SessionPendingOTPUsername)
		qrCodeDataURL, _ := utils.GenerateOTPQRCodeDataURL(otpAuthURL, 240)
		c.HTML(http.StatusBadRequest, "auth/setup_2fa.html", gin.H{
			"Title":         "设置 2FA",
			"Error":         "验证码不正确，请重试。",
			"Secret":        secret,
			"OtpAuthURL":    otpAuthURL,
			"AccountName":   accountName,
			"QRCodeDataURL": safeTemplateDataImageURL(qrCodeDataURL),
		})
		return
	}

	if err := a.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("otp_secret", secret).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "auth/setup_2fa.html", gin.H{
			"Title": "设置 2FA",
			"Error": "保存 2FA 配置失败。",
		})
		return
	}

	session.Delete(middleware.SessionPendingOTPSecret)
	session.Delete(middleware.SessionPendingOTPAUTHURL)
	session.Delete(middleware.SessionPendingOTPUsername)
	session.Set(middleware.Session2FAVerifiedKey, true)
	_ = session.Save()

	c.Redirect(http.StatusFound, "/dashboard")
}

func sessionStringValue(session sessions.Session, key string) string {
	raw := session.Get(key)
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func safeTemplateDataImageURL(raw string) htemplate.URL {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "data:image/") {
		return htemplate.URL("")
	}
	return htemplate.URL(value)
}

func (a *AppContext) showVerify2FA(c *gin.Context) {
	user, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if strings.TrimSpace(user.OTPSecret) == "" {
		c.Redirect(http.StatusFound, "/auth/setup-2fa")
		return
	}
	c.HTML(http.StatusOK, "auth/verify_2fa.html", gin.H{
		"Title": "验证 2FA",
		"Error": c.Query("error"),
	})
}

func (a *AppContext) verify2FA(c *gin.Context) {
	user, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if strings.TrimSpace(user.OTPSecret) == "" {
		c.Redirect(http.StatusFound, "/auth/setup-2fa")
		return
	}

	session := sessions.Default(c)
	maxAttempts := middleware.ParsePositiveInt(a.ConfigCenter.Get("TOTP_VERIFY_MAX_ATTEMPTS", "5"), 5)
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	blockSeconds := middleware.ParsePositiveInt(a.ConfigCenter.Get("TOTP_VERIFY_BLOCK_SECONDS", "300"), 300)
	if blockSeconds <= 0 {
		blockSeconds = 300
	}
	blockDuration := time.Duration(blockSeconds) * time.Second

	now := time.Now()
	blockedUntil := sessionUnixTimeValue(session.Get(middleware.Session2FABlockedUntil))
	if !blockedUntil.IsZero() && now.Before(blockedUntil) {
		retryAfterSeconds := int(blockedUntil.Sub(now).Seconds()) + 1
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
		c.HTML(http.StatusTooManyRequests, "auth/verify_2fa.html", gin.H{
			"Title": "验证 2FA",
			"Error": fmt.Sprintf("验证码错误次数过多，请在 %d 秒后重试。", retryAfterSeconds),
		})
		return
	}

	code := strings.TrimSpace(c.PostForm("code"))
	if !totp.Validate(code, user.OTPSecret) {
		failedAttempts := sessionIntValue(session.Get(middleware.Session2FAFailedAttempts)) + 1
		statusCode := http.StatusUnauthorized
		errorMessage := "验证码不正确。"
		if failedAttempts >= maxAttempts {
			blockedUntil = now.Add(blockDuration)
			session.Set(middleware.Session2FAFailedAttempts, 0)
			session.Set(middleware.Session2FABlockedUntil, blockedUntil.Unix())
			retryAfterSeconds := int(blockDuration.Seconds())
			if retryAfterSeconds < 1 {
				retryAfterSeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
			statusCode = http.StatusTooManyRequests
			errorMessage = fmt.Sprintf("验证码错误次数过多，请在 %d 秒后重试。", retryAfterSeconds)
		} else {
			session.Set(middleware.Session2FAFailedAttempts, failedAttempts)
		}
		_ = session.Save()
		c.HTML(statusCode, "auth/verify_2fa.html", gin.H{
			"Title": "验证 2FA",
			"Error": errorMessage,
		})
		return
	}

	session.Delete(middleware.Session2FAFailedAttempts)
	session.Delete(middleware.Session2FABlockedUntil)
	session.Set(middleware.Session2FAVerifiedKey, true)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusInternalServerError, "auth/verify_2fa.html", gin.H{
			"Title": "验证 2FA",
			"Error": "会话更新失败，请重试。",
		})
		return
	}

	c.Redirect(http.StatusFound, "/dashboard")
}

func sessionIntValue(raw any) int {
	switch v := raw.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

func sessionUnixTimeValue(raw any) time.Time {
	seconds := int64(sessionIntValue(raw))
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}
