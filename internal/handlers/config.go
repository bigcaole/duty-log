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
	Key          string
	Description  string
	IsSensitive  bool
	DefaultValue string
	Required     bool
}

var systemConfigDefinitions = []systemConfigDefinition{
	{Key: "AI_API_KEY", Description: "AI API 密钥（加密存储，仅管理员用于生成报表摘要）", IsSensitive: true},
	{Key: "AI_API_BASE", Description: "AI API 地址（仅管理员报表摘要使用）"},
	{Key: "AI_MODEL", Description: "AI 模型名称（仅管理员报表摘要使用）"},
	{Key: "MAIL_SERVER", Description: "SMTP 服务器"},
	{Key: "MAIL_PORT", Description: "SMTP 端口"},
	{Key: "MAIL_USE_TLS", Description: "是否启用 TLS"},
	{Key: "MAIL_USERNAME", Description: "SMTP 用户名"},
	{Key: "MAIL_PASSWORD", Description: "SMTP 密码（加密存储）", IsSensitive: true},
	{Key: "MAIL_DEFAULT_SENDER", Description: "默认发件人"},
	{Key: "FEISHU_WEBHOOK_URL", Description: "飞书 Webhook（加密存储）", IsSensitive: true},
	{Key: "BACKUP_ENABLED", Description: "是否启用自动备份", DefaultValue: "false", Required: true},
	{Key: "BACKUP_EMAIL", Description: "备份接收邮箱"},
	{Key: "BACKUP_SCHEDULE", Description: "备份执行时间（每周）", DefaultValue: "0 18 * * 0"},
	{Key: "BACKUP_RETENTION_DAYS", Description: "备份保留天数", DefaultValue: "30", Required: true},
	{Key: "REPORT_FEISHU_ENABLED", Description: "是否启用自动报表飞书推送", DefaultValue: "false"},
	{Key: "REPORT_WEEKLY_WEEKDAY", Description: "周报推送星期（0=周日）", DefaultValue: "0"},
	{Key: "REPORT_WEEKLY_TIME", Description: "周报推送时间（HH:MM）", DefaultValue: "17:00"},
	{Key: "REPORT_MONTHLY_DAY", Description: "月报推送日期（1-31 或 last）", DefaultValue: "last"},
	{Key: "REPORT_MONTHLY_TIME", Description: "月报推送时间（HH:MM）", DefaultValue: "17:00"},
	{Key: "REPORT_HALFYEAR_DAY1", Description: "半年报推送日期 1（6 月）", DefaultValue: "25"},
	{Key: "REPORT_HALFYEAR_DAY2", Description: "半年报推送日期 2（6 月）", DefaultValue: "30"},
	{Key: "REPORT_HALFYEAR_TIME", Description: "半年报推送时间（HH:MM）", DefaultValue: "17:00"},
	{Key: "REPORT_YEAR_DAY1", Description: "年报推送日期 1（12 月）", DefaultValue: "25"},
	{Key: "REPORT_YEAR_DAY2", Description: "年报推送日期 2（12 月）", DefaultValue: "31"},
	{Key: "REPORT_YEAR_TIME", Description: "年报推送时间（HH:MM）", DefaultValue: "17:00"},
	{Key: "REMINDER_FEISHU_ENABLED", Description: "是否启用提醒飞书推送", DefaultValue: "false"},
	{Key: "AUDIT_RETENTION_DAYS", Description: "审计日志保留天数", DefaultValue: "90", Required: true},
	{Key: "LOGIN_MAX_ATTEMPTS", Description: "登录窗口最大失败次数", DefaultValue: "5", Required: true},
	{Key: "LOGIN_WINDOW_SECONDS", Description: "登录失败统计窗口秒数", DefaultValue: "600", Required: true},
	{Key: "LOGIN_BLOCK_SECONDS", Description: "登录临时封禁秒数", DefaultValue: "900", Required: true},
	{Key: "TOTP_ISSUER", Description: "2FA 发行方名称", DefaultValue: "Duty-Log-System", Required: true},
	{Key: "TOTP_VERIFY_MAX_ATTEMPTS", Description: "2FA 验证最大失败次数", DefaultValue: "5"},
	{Key: "TOTP_VERIFY_BLOCK_SECONDS", Description: "2FA 验证临时封禁秒数", DefaultValue: "300"},
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
	group.POST("/config/test-email", app.testEmailConfig)
	group.POST("/config/test-feishu", app.testFeishuConfig)
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
		rawValue := a.ConfigCenter.Get(configItems[i].Key, defaultValueForSystemConfigKey(configItems[i].Key))
		configItems[i].Value = rawValue
		configItems[i].DisplayValue = rawValue
		if configItems[i].IsSensitive && !revealSensitive {
			configItems[i].Value = ""
			configItems[i].DisplayValue = maskSystemConfigValue(rawValue)
		}
		configItems[i].Source = a.ConfigCenter.Source(configItems[i].Key)
	}

	backupSchedule := parseBackupSchedule(a.ConfigCenter.Get("BACKUP_SCHEDULE", defaultValueForSystemConfigKey("BACKUP_SCHEDULE")))
	reportSchedule := parseReportSchedule(a.ConfigCenter)
	hourOptions := make([]int, 0, 24)
	for i := 0; i < 24; i++ {
		hourOptions = append(hourOptions, i)
	}
	minuteOptions := []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55}
	dayOptions := make([]int, 0, 31)
	for i := 1; i <= 31; i++ {
		dayOptions = append(dayOptions, i)
	}
	monthOptions := make([]int, 0, 12)
	for i := 1; i <= 12; i++ {
		monthOptions = append(monthOptions, i)
	}

	c.HTML(http.StatusOK, "admin/config.html", gin.H{
		"Title":            "系统配置",
		"Items":            configItems,
		"Msg":              strings.TrimSpace(c.Query("msg")),
		"Error":            strings.TrimSpace(c.Query("error")),
		"RevealSensitive":  revealSensitive,
		"TestEmailDefault": a.ConfigCenter.Get("BACKUP_EMAIL", ""),
		"BackupSchedule":   backupSchedule,
		"ReportSchedule":   reportSchedule,
		"HourOptions":      hourOptions,
		"MinuteOptions":    minuteOptions,
		"DayOptions":       dayOptions,
		"MonthOptions":     monthOptions,
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
	reportNeedsReload := hasUpdatedSystemConfigKey(updates, isReportSchedulerRelatedKey)
	reminderNeedsReload := hasUpdatedSystemConfigKey(updates, isReminderSchedulerRelatedKey)

	if backupSchedulerNeedsReload {
		if err := a.ReloadBackupScheduler(); err != nil {
			c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("备份调度器重载失败："+err.Error()))
			return
		}
	}

	if loginRateLimitNeedsReload {
		applyLoginRateLimiterConfig(a.ConfigCenter)
	}
	if reportNeedsReload {
		if err := a.ReloadReportScheduler(); err != nil {
			c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("报表调度器重载失败："+err.Error()))
			return
		}
	}
	if reminderNeedsReload {
		if err := a.ReloadReminderScheduler(); err != nil {
			c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("提醒调度器重载失败："+err.Error()))
			return
		}
	}

	msgParts := []string{"配置已保存"}
	if backupSchedulerNeedsReload {
		msgParts = append(msgParts, "备份调度器已重载")
	}
	if loginRateLimitNeedsReload {
		msgParts = append(msgParts, "登录限流配置已重载")
	}
	if reportNeedsReload {
		msgParts = append(msgParts, "报表调度器已重载")
	}
	if reminderNeedsReload {
		msgParts = append(msgParts, "提醒调度器已重载")
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
	if isReportSchedulerRelatedKey(key) {
		if err := a.ReloadReportScheduler(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "配置已保存，但报表调度器重载失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "配置已保存，报表调度器已重载"})
		return
	}
	if isReminderSchedulerRelatedKey(key) {
		if err := a.ReloadReminderScheduler(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "配置已保存，但提醒调度器重载失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "配置已保存，提醒调度器已重载"})
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

func defaultValueForSystemConfigKey(key string) string {
	for _, def := range systemConfigDefinitions {
		if strings.TrimSpace(def.Key) == strings.TrimSpace(key) {
			return strings.TrimSpace(def.DefaultValue)
		}
	}
	return ""
}

func requiredSetupConfigDefinitions() []systemConfigDefinition {
	defs := make([]systemConfigDefinition, 0, len(systemConfigDefinitions))
	for _, def := range systemConfigDefinitions {
		if def.Required {
			defs = append(defs, def)
		}
	}
	return defs
}

func optionalSetupConfigDefinitions() []systemConfigDefinition {
	defs := make([]systemConfigDefinition, 0, len(systemConfigDefinitions))
	for _, def := range systemConfigDefinitions {
		if !def.Required {
			defs = append(defs, def)
		}
	}
	return defs
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
	case "BACKUP_ENABLED", "BACKUP_SCHEDULE", "BACKUP_EMAIL", "BACKUP_RETENTION_DAYS":
		return true
	default:
		return false
	}
}

func isReportSchedulerRelatedKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "REPORT_FEISHU_ENABLED", "FEISHU_WEBHOOK_URL",
		"REPORT_WEEKLY_WEEKDAY", "REPORT_WEEKLY_TIME",
		"REPORT_MONTHLY_DAY", "REPORT_MONTHLY_TIME",
		"REPORT_HALFYEAR_DAY1", "REPORT_HALFYEAR_DAY2", "REPORT_HALFYEAR_TIME",
		"REPORT_YEAR_DAY1", "REPORT_YEAR_DAY2", "REPORT_YEAR_TIME":
		return true
	default:
		return false
	}
}

func isReminderSchedulerRelatedKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "REMINDER_FEISHU_ENABLED", "FEISHU_WEBHOOK_URL":
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
	case "BACKUP_ENABLED", "MAIL_USE_TLS", "REPORT_FEISHU_ENABLED", "REMINDER_FEISHU_ENABLED":
		return normalizeBoolConfigValue(k, v)
	case "MAIL_PORT":
		return normalizeIntConfigValue(k, v, 1, 65535)
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
	case "REPORT_WEEKLY_WEEKDAY":
		return normalizeIntConfigValue(k, v, 0, 6)
	case "REPORT_MONTHLY_DAY":
		return normalizeMonthlyDayValue(k, v)
	case "REPORT_HALFYEAR_DAY1", "REPORT_HALFYEAR_DAY2", "REPORT_YEAR_DAY1", "REPORT_YEAR_DAY2":
		return normalizeIntConfigValue(k, v, 1, 31)
	case "REPORT_WEEKLY_TIME", "REPORT_MONTHLY_TIME", "REPORT_HALFYEAR_TIME", "REPORT_YEAR_TIME":
		return normalizeTimeConfigValue(k, v)
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

func normalizeTimeConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("%s 必须为 HH:MM 格式", key)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "", fmt.Errorf("%s 小时必须在 0-23", key)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return "", fmt.Errorf("%s 分钟必须在 0-59", key)
	}
	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

func normalizeMonthlyDayValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "last" {
		return "last", nil
	}
	return normalizeIntConfigValue(key, trimmed, 1, 31)
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

func normalizeEmailListConfigValue(key, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		email, err := normalizeEmailConfigValue(key, part)
		if err != nil {
			return "", err
		}
		if email != "" {
			result = append(result, email)
		}
	}
	return strings.Join(result, ","), nil
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

func (a *AppContext) testEmailConfig(c *gin.Context) {
	recipientRaw := strings.TrimSpace(c.PostForm("recipient"))
	recipient, err := normalizeEmailConfigValue("recipient", recipientRaw)
	if err != nil || recipient == "" {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("测试邮箱格式不合法"))
		return
	}

	smtpConfig := utils.SMTPConfig{
		Server:        a.ConfigCenter.Get("MAIL_SERVER", "smtp.gmail.com"),
		Port:          a.ConfigCenter.GetInt("MAIL_PORT", 587),
		UseTLS:        a.ConfigCenter.GetBool("MAIL_USE_TLS", true),
		Username:      a.ConfigCenter.Get("MAIL_USERNAME", ""),
		Password:      a.ConfigCenter.Get("MAIL_PASSWORD", ""),
		DefaultSender: a.ConfigCenter.Get("MAIL_DEFAULT_SENDER", ""),
	}

	subject := "Duty-Log 邮件配置测试"
	body := fmt.Sprintf("测试时间：%s\n如果你收到这封邮件，说明 SMTP 配置可用。", time.Now().Format("2006-01-02 15:04:05"))
	if sendErr := utils.SendEmail(smtpConfig, []string{recipient}, subject, body, "", nil); sendErr != nil {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("邮件测试失败："+sendErr.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/admin/config?msg="+url.QueryEscape("邮件测试发送成功，请检查收件箱"))
}

func (a *AppContext) testFeishuConfig(c *gin.Context) {
	webhook := strings.TrimSpace(a.ConfigCenter.Get("FEISHU_WEBHOOK_URL", ""))
	if webhook == "" {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("FEISHU_WEBHOOK_URL 未配置"))
		return
	}
	content := fmt.Sprintf("测试时间：%s\n如果你看到该消息，说明飞书 Webhook 配置可用。", time.Now().Format("2006-01-02 15:04:05"))
	if err := utils.SendFeishuText(webhook, "Duty-Log 飞书配置测试", content); err != nil {
		c.Redirect(http.StatusFound, "/admin/config?error="+url.QueryEscape("飞书测试失败："+err.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/admin/config?msg="+url.QueryEscape("飞书推送测试成功"))
}

func collectBulkSystemConfigUpdates(postForm func(string) string) ([]systemConfigUpdate, bool, error) {
	keys := systemConfigKeys()
	updates := make([]systemConfigUpdate, 0, len(keys))
	needsReload := false

	for _, key := range keys {
		value := strings.TrimSpace(postForm(key))
		if key == "BACKUP_SCHEDULE" {
			value = composeBackupSchedule(
				postForm("backup_schedule_type"),
				postForm("backup_weekday"),
				postForm("backup_hour"),
				postForm("backup_minute"),
				postForm("backup_month_day"),
				postForm("backup_month"),
			)
		}
		switch key {
		case "REPORT_WEEKLY_WEEKDAY":
			value = strings.TrimSpace(postForm("report_weekly_weekday"))
		case "REPORT_WEEKLY_TIME":
			value = composeClock(postForm("report_weekly_hour"), postForm("report_weekly_minute"), "17", "0")
		case "REPORT_MONTHLY_DAY":
			value = strings.TrimSpace(postForm("report_monthly_day"))
		case "REPORT_MONTHLY_TIME":
			value = composeClock(postForm("report_monthly_hour"), postForm("report_monthly_minute"), "17", "0")
		case "REPORT_HALFYEAR_DAY1":
			value = strings.TrimSpace(postForm("report_halfyear_day1"))
		case "REPORT_HALFYEAR_DAY2":
			value = strings.TrimSpace(postForm("report_halfyear_day2"))
		case "REPORT_HALFYEAR_TIME":
			value = composeClock(postForm("report_halfyear_hour"), postForm("report_halfyear_minute"), "17", "0")
		case "REPORT_YEAR_DAY1":
			value = strings.TrimSpace(postForm("report_year_day1"))
		case "REPORT_YEAR_DAY2":
			value = strings.TrimSpace(postForm("report_year_day2"))
		case "REPORT_YEAR_TIME":
			value = composeClock(postForm("report_year_hour"), postForm("report_year_minute"), "17", "0")
		}
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

func composeBackupSchedule(typeRaw, weekdayRaw, hourRaw, minuteRaw, monthDayRaw, monthRaw string) string {
	scheduleType := strings.ToLower(strings.TrimSpace(typeRaw))
	weekday := strings.TrimSpace(weekdayRaw)
	hour := strings.TrimSpace(hourRaw)
	minute := strings.TrimSpace(minuteRaw)
	monthDay := strings.TrimSpace(monthDayRaw)
	month := strings.TrimSpace(monthRaw)

	if hour == "" {
		hour = "18"
	}
	if minute == "" {
		minute = "0"
	}

	switch scheduleType {
	case "daily":
		return fmt.Sprintf("%s %s * * *", minute, hour)
	case "monthly":
		if monthDay == "" {
			monthDay = "1"
		}
		return fmt.Sprintf("%s %s %s * *", minute, hour, monthDay)
	case "yearly":
		if monthDay == "" {
			monthDay = "1"
		}
		if month == "" {
			month = "1"
		}
		return fmt.Sprintf("%s %s %s %s *", minute, hour, monthDay, month)
	default:
		if weekday == "" {
			weekday = "0"
		}
		return fmt.Sprintf("%s %s * * %s", minute, hour, weekday)
	}
}

func parseBackupSchedule(raw string) map[string]string {
	minute := "0"
	hour := "18"
	weekday := "0"
	monthDay := "1"
	month := "1"
	scheduleType := "weekly"

	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 5 {
		if fields[0] != "" {
			minute = fields[0]
		}
		if fields[1] != "" {
			hour = fields[1]
		}
		if fields[2] != "" {
			monthDay = fields[2]
		}
		if fields[3] != "" {
			month = fields[3]
		}
		if fields[4] != "" {
			weekday = fields[4]
		}
		switch {
		case fields[4] != "*" && fields[2] == "*":
			scheduleType = "weekly"
		case fields[3] != "*":
			scheduleType = "yearly"
		case fields[2] != "*":
			scheduleType = "monthly"
		default:
			scheduleType = "daily"
		}
	}

	return map[string]string{
		"Minute":   minute,
		"Hour":     hour,
		"Weekday":  weekday,
		"Month":    month,
		"MonthDay": monthDay,
		"Type":     scheduleType,
	}
}

func composeClock(hourRaw, minuteRaw, defaultHour, defaultMinute string) string {
	hour := strings.TrimSpace(hourRaw)
	minute := strings.TrimSpace(minuteRaw)
	if hour == "" {
		hour = defaultHour
	}
	if minute == "" {
		minute = defaultMinute
	}
	h, err := strconv.Atoi(hour)
	if err != nil {
		h = 0
	}
	m, err := strconv.Atoi(minute)
	if err != nil {
		m = 0
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

func parseClockParts(raw string, defaultHour, defaultMinute string) map[string]string {
	clock := strings.TrimSpace(raw)
	if clock == "" {
		return map[string]string{
			"Hour":   defaultHour,
			"Minute": defaultMinute,
		}
	}
	parts := strings.Split(clock, ":")
	if len(parts) != 2 {
		return map[string]string{
			"Hour":   defaultHour,
			"Minute": defaultMinute,
		}
	}
	hour := strings.TrimSpace(parts[0])
	minute := strings.TrimSpace(parts[1])
	if hour == "" {
		hour = defaultHour
	}
	if minute == "" {
		minute = defaultMinute
	}
	return map[string]string{
		"Hour":   hour,
		"Minute": minute,
	}
}

func parseReportSchedule(configCenter *utils.ConfigCenter) map[string]string {
	weeklyWeekday := strings.TrimSpace(configCenter.Get("REPORT_WEEKLY_WEEKDAY", defaultValueForSystemConfigKey("REPORT_WEEKLY_WEEKDAY")))
	weeklyTime := parseClockParts(configCenter.Get("REPORT_WEEKLY_TIME", defaultValueForSystemConfigKey("REPORT_WEEKLY_TIME")), "17", "00")
	monthlyDay := strings.TrimSpace(configCenter.Get("REPORT_MONTHLY_DAY", defaultValueForSystemConfigKey("REPORT_MONTHLY_DAY")))
	monthlyTime := parseClockParts(configCenter.Get("REPORT_MONTHLY_TIME", defaultValueForSystemConfigKey("REPORT_MONTHLY_TIME")), "17", "00")
	halfDay1 := strings.TrimSpace(configCenter.Get("REPORT_HALFYEAR_DAY1", defaultValueForSystemConfigKey("REPORT_HALFYEAR_DAY1")))
	halfDay2 := strings.TrimSpace(configCenter.Get("REPORT_HALFYEAR_DAY2", defaultValueForSystemConfigKey("REPORT_HALFYEAR_DAY2")))
	halfTime := parseClockParts(configCenter.Get("REPORT_HALFYEAR_TIME", defaultValueForSystemConfigKey("REPORT_HALFYEAR_TIME")), "17", "00")
	yearDay1 := strings.TrimSpace(configCenter.Get("REPORT_YEAR_DAY1", defaultValueForSystemConfigKey("REPORT_YEAR_DAY1")))
	yearDay2 := strings.TrimSpace(configCenter.Get("REPORT_YEAR_DAY2", defaultValueForSystemConfigKey("REPORT_YEAR_DAY2")))
	yearTime := parseClockParts(configCenter.Get("REPORT_YEAR_TIME", defaultValueForSystemConfigKey("REPORT_YEAR_TIME")), "17", "00")

	if weeklyWeekday == "" {
		weeklyWeekday = "0"
	}
	if monthlyDay == "" {
		monthlyDay = "last"
	}

	return map[string]string{
		"WeeklyWeekday": weeklyWeekday,
		"WeeklyHour":    weeklyTime["Hour"],
		"WeeklyMinute":  weeklyTime["Minute"],
		"MonthlyDay":    monthlyDay,
		"MonthlyHour":   monthlyTime["Hour"],
		"MonthlyMinute": monthlyTime["Minute"],
		"HalfDay1":      halfDay1,
		"HalfDay2":      halfDay2,
		"HalfHour":      halfTime["Hour"],
		"HalfMinute":    halfTime["Minute"],
		"YearDay1":      yearDay1,
		"YearDay2":      yearDay2,
		"YearHour":      yearTime["Hour"],
		"YearMinute":    yearTime["Minute"],
	}
}
