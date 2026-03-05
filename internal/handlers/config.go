package handlers

import (
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/pkg/utils"

	"github.com/robfig/cron/v3"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type systemConfigDefinition struct {
	Key         string
	Description string
	IsSensitive bool
}

var systemConfigDefinitions = []systemConfigDefinition{
	{Key: "AI_API_KEY", Description: "AI API 密钥（加密存储）", IsSensitive: true},
	{Key: "AI_API_BASE", Description: "AI API 地址"},
	{Key: "AI_MODEL", Description: "AI 模型名称"},
	{Key: "MAIL_SERVER", Description: "SMTP 服务器"},
	{Key: "MAIL_PORT", Description: "SMTP 端口"},
	{Key: "MAIL_USE_TLS", Description: "是否启用 TLS"},
	{Key: "MAIL_USERNAME", Description: "SMTP 用户名"},
	{Key: "MAIL_PASSWORD", Description: "SMTP 密码（加密存储）", IsSensitive: true},
	{Key: "MAIL_DEFAULT_SENDER", Description: "默认发件人"},
	{Key: "FEISHU_WEBHOOK_URL", Description: "飞书 Webhook（加密存储）", IsSensitive: true},
	{Key: "BACKUP_ENABLED", Description: "是否启用自动备份"},
	{Key: "BACKUP_EMAIL", Description: "备份接收邮箱"},
	{Key: "BACKUP_HOUR", Description: "备份小时（0-23）"},
	{Key: "BACKUP_MINUTE", Description: "备份分钟（0-59）"},
	{Key: "BACKUP_SCHEDULE", Description: "备份 Cron 表达式"},
	{Key: "BACKUP_RETENTION_DAYS", Description: "备份保留天数"},
	{Key: "AUDIT_RETENTION_DAYS", Description: "审计日志保留天数"},
	{Key: "LOGIN_MAX_ATTEMPTS", Description: "登录窗口最大失败次数"},
	{Key: "LOGIN_WINDOW_SECONDS", Description: "登录失败统计窗口秒数"},
	{Key: "LOGIN_BLOCK_SECONDS", Description: "登录临时封禁秒数"},
	{Key: "TOTP_ISSUER", Description: "2FA 发行方名称"},
	{Key: "TOTP_VERIFY_MAX_ATTEMPTS", Description: "2FA 验证最大失败次数"},
	{Key: "TOTP_VERIFY_BLOCK_SECONDS", Description: "2FA 验证临时封禁秒数"},
}

var sensitiveSystemConfigKeys = buildSensitiveSystemConfigKeys(systemConfigDefinitions)

type systemConfigItem struct {
	Key          string
	Value        string
	DisplayValue string
	Description  string
	IsSensitive  bool
	Source       string
}

type systemConfigUpdate struct {
	Key   string
	Value string
}

func registerSystemConfigRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/config", app.showSystemConfigPage)
	group.POST("/config", app.saveSystemConfig)
}

func (a *AppContext) showSystemConfigPage(c *gin.Context) {
	revealSensitive := parseBoolQuery(c.Query("reveal"))

	configItems := make([]systemConfigItem, 0, len(systemConfigDefinitions))
	for _, def := range systemConfigDefinitions {
		configItems = append(configItems, systemConfigItem{
			Key:         def.Key,
			Description: def.Description,
			IsSensitive: def.IsSensitive,
		})
	}

	for i := range configItems {
		rawValue := a.ConfigCenter.Get(configItems[i].Key, "")
		configItems[i].Value = rawValue
		configItems[i].DisplayValue = rawValue
		if configItems[i].IsSensitive && !revealSensitive {
			configItems[i].Value = ""
			configItems[i].DisplayValue = maskSystemConfigValue(rawValue)
		}
		configItems[i].Source = a.ConfigCenter.Source(configItems[i].Key)
	}

	c.HTML(http.StatusOK, "admin/config.html", gin.H{
		"Title":           "系统配置",
		"Items":           configItems,
		"Msg":             strings.TrimSpace(c.Query("msg")),
		"Error":           strings.TrimSpace(c.Query("error")),
		"RevealSensitive": revealSensitive,
	})
}

func (a *AppContext) saveSystemConfig(c *gin.Context) {
	saveMode := strings.TrimSpace(c.PostForm("mode"))
	if saveMode == "single" {
		a.saveSingleSystemConfig(c)
		return
	}

	updates, backupSchedulerNeedsReload, err := collectBulkSystemConfigUpdates(c.PostForm)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape(err.Error()))
		return
	}

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		txConfigCenter := utils.NewConfigCenter(tx, a.Config.SecretKey)
		for _, update := range updates {
			if upsertErr := txConfigCenter.Upsert(update.Key, update.Value, "updated from admin panel"); upsertErr != nil {
				return upsertErr
			}
		}
		return nil
	}); err != nil {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape(err.Error()))
		return
	}

	loginRateLimitNeedsReload := hasUpdatedSystemConfigKey(updates, isLoginRateLimitRelatedKey)

	if backupSchedulerNeedsReload {
		if err := a.ReloadBackupScheduler(); err != nil {
			c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("备份调度器重载失败："+err.Error()))
			return
		}
	}

	if loginRateLimitNeedsReload {
		applyLoginRateLimiterConfig(a.ConfigCenter)
	}

	msgParts := []string{"配置已保存"}
	if backupSchedulerNeedsReload {
		msgParts = append(msgParts, "备份调度器已重载")
	}
	if loginRateLimitNeedsReload {
		msgParts = append(msgParts, "登录限流配置已重载")
	}
	c.Redirect(http.StatusFound, "/admin/config?msg="+url.QueryEscape(strings.Join(msgParts, "，")))
}

func (a *AppContext) saveSingleSystemConfig(c *gin.Context) {
	key := strings.TrimSpace(c.PostForm("key"))
	value := strings.TrimSpace(c.PostForm("value"))
	desc := strings.TrimSpace(c.PostForm("description"))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "key is required"})
		return
	}
	if shouldSkipSensitiveBulkUpdate(key, value) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "敏感值留空，保持原配置不变"})
		return
	}

	normalized, err := normalizeSystemConfigValue(key, value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}

	if err := a.ConfigCenter.Upsert(key, normalized, desc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if isLoginRateLimitRelatedKey(key) {
		applyLoginRateLimiterConfig(a.ConfigCenter)
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "配置已保存，登录限流配置已重载"})
		return
	}
	if isBackupSchedulerRelatedKey(key) {
		if err := a.ReloadBackupScheduler(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "配置已保存，但备份调度器重载失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "配置已保存，备份调度器已重载"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func isSensitiveSystemConfigKey(key string) bool {
	_, ok := sensitiveSystemConfigKeys[strings.TrimSpace(key)]
	return ok
}

func systemConfigKeys() []string {
	keys := make([]string, 0, len(systemConfigDefinitions))
	for _, def := range systemConfigDefinitions {
		keys = append(keys, def.Key)
	}
	return keys
}

func buildSensitiveSystemConfigKeys(defs []systemConfigDefinition) map[string]struct{} {
	keys := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		if def.IsSensitive {
			keys[strings.TrimSpace(def.Key)] = struct{}{}
		}
	}
	return keys
}

func shouldSkipSensitiveBulkUpdate(key, value string) bool {
	return isSensitiveSystemConfigKey(key) && strings.TrimSpace(value) == ""
}

func isBackupSchedulerRelatedKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "BACKUP_ENABLED", "BACKUP_HOUR", "BACKUP_MINUTE", "BACKUP_SCHEDULE", "BACKUP_EMAIL", "BACKUP_RETENTION_DAYS":
		return true
	default:
		return false
	}
}

func isLoginRateLimitRelatedKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "LOGIN_MAX_ATTEMPTS", "LOGIN_WINDOW_SECONDS", "LOGIN_BLOCK_SECONDS":
		return true
	default:
		return false
	}
}

func hasUpdatedSystemConfigKey(updates []systemConfigUpdate, match func(string) bool) bool {
	for _, update := range updates {
		if match(update.Key) {
			return true
		}
	}
	return false
}

func applyLoginRateLimiterConfig(configCenter *utils.ConfigCenter) {
	maxAttempts := middleware.ParsePositiveInt(configCenter.Get("LOGIN_MAX_ATTEMPTS", "5"), 5)
	windowSeconds := middleware.ParsePositiveInt(configCenter.Get("LOGIN_WINDOW_SECONDS", "600"), 600)
	blockSeconds := middleware.ParsePositiveInt(configCenter.Get("LOGIN_BLOCK_SECONDS", "900"), 900)
	middleware.ConfigureLoginRateLimiter(
		maxAttempts,
		time.Duration(windowSeconds)*time.Second,
		time.Duration(blockSeconds)*time.Second,
	)
}

func maskSystemConfigValue(value string) string {
	return maskBackupPassword(value)
}

func normalizeSystemConfigValue(key, value string) (string, error) {
	k := strings.TrimSpace(key)
	v := strings.TrimSpace(value)

	switch k {
	case "BACKUP_ENABLED", "MAIL_USE_TLS":
		return normalizeBoolConfigValue(k, v)
	case "MAIL_PORT":
		return normalizeIntConfigValue(k, v, 1, 65535)
	case "BACKUP_HOUR":
		return normalizeIntConfigValue(k, v, 0, 23)
	case "BACKUP_MINUTE":
		return normalizeIntConfigValue(k, v, 0, 59)
	case "BACKUP_RETENTION_DAYS", "AUDIT_RETENTION_DAYS":
		return normalizeIntConfigValue(k, v, 1, 3650)
	case "LOGIN_MAX_ATTEMPTS":
		return normalizeIntConfigValue(k, v, 1, 100)
	case "LOGIN_WINDOW_SECONDS", "LOGIN_BLOCK_SECONDS":
		return normalizeIntConfigValue(k, v, 1, 86400)
	case "TOTP_VERIFY_MAX_ATTEMPTS":
		return normalizeIntConfigValue(k, v, 1, 20)
	case "TOTP_VERIFY_BLOCK_SECONDS":
		return normalizeIntConfigValue(k, v, 1, 86400)
	case "BACKUP_SCHEDULE":
		return normalizeCronConfigValue(k, v)
	case "BACKUP_EMAIL":
		return normalizeEmailConfigValue(k, v)
	case "AI_API_BASE":
		return normalizeURLConfigValue(k, v)
	default:
		return v, nil
	}
}

func normalizeBoolConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := strconv.ParseBool(strings.ToLower(value))
	if err != nil {
		return "", fmt.Errorf("%s 必须为 true/false", key)
	}
	if parsed {
		return "true", nil
	}
	return "false", nil
}

func normalizeIntConfigValue(key, value string, min, max int) (string, error) {
	if value == "" {
		return "", nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("%s 必须为整数", key)
	}
	if n < min || n > max {
		return "", fmt.Errorf("%s 必须在 %d 到 %d 之间", key, min, max)
	}
	return strconv.Itoa(n), nil
}

func normalizeCronConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	fields := strings.Fields(value)
	normalized := strings.Join(fields, " ")
	if _, err := cron.ParseStandard(normalized); err != nil {
		return "", fmt.Errorf("%s 不是合法的 5 段 cron 表达式", key)
	}
	return normalized, nil
}

func normalizeEmailConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || strings.TrimSpace(addr.Address) == "" {
		return "", fmt.Errorf("%s 邮箱格式不合法", key)
	}
	return strings.TrimSpace(addr.Address), nil
}

func normalizeURLConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%s URL 格式不合法", key)
	}
	return parsed.String(), nil
}

func collectBulkSystemConfigUpdates(postForm func(string) string) ([]systemConfigUpdate, bool, error) {
	keys := systemConfigKeys()
	updates := make([]systemConfigUpdate, 0, len(keys))
	needsReload := false

	for _, key := range keys {
		value := strings.TrimSpace(postForm(key))
		if shouldSkipSensitiveBulkUpdate(key, value) {
			continue
		}
		normalized, err := normalizeSystemConfigValue(key, value)
		if err != nil {
			return nil, false, err
		}
		updates = append(updates, systemConfigUpdate{
			Key:   key,
			Value: normalized,
		})
		if isBackupSchedulerRelatedKey(key) {
			needsReload = true
		}
	}

	return updates, needsReload, nil
}
