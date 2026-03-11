package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"
	"duty-log-system/internal/scheduler"
	"duty-log-system/pkg/utils"

	"github.com/gin-gonic/gin"
)

type adminBackupNotificationItem struct {
	ID             uint
	BackupFilePath string
	BackupPassword string
	RecipientEmail string
	SentAt         *time.Time
	CreatedAt      time.Time
}

func (a *AppContext) adminAuditLogList(c *gin.Context) {
	action := strings.TrimSpace(c.Query("action"))
	pathKeyword := strings.TrimSpace(c.Query("path"))

	query := a.DB.Model(&models.AuditLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if pathKeyword != "" {
		query = query.Where("details_json::text ILIKE ?", "%"+pathKeyword+"%")
	}

	var records []models.AuditLog
	if err := query.Order("created_at desc").Limit(300).Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "审计日志",
			"Path":    "/admin/audit-logs",
			"Message": "读取审计日志失败：" + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "admin/audit_logs.html", gin.H{
		"Title":        "审计日志",
		"Items":        records,
		"FilterAction": action,
		"FilterPath":   pathKeyword,
		"Msg":          strings.TrimSpace(c.Query("msg")),
		"Error":        strings.TrimSpace(c.Query("error")),
		"Retention":    a.ConfigCenter.GetInt("AUDIT_RETENTION_DAYS", 90),
	})
}

func (a *AppContext) adminBackupNotificationList(c *gin.Context) {
	revealPasswords := parseBoolQuery(c.Query("reveal"))

	var records []models.BackupNotification
	if err := a.DB.Order("created_at desc").Limit(200).Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "备份通知",
			"Path":    "/admin/backup-notifications",
			"Message": "读取备份通知失败：" + err.Error(),
		})
		return
	}

	items := make([]adminBackupNotificationItem, 0, len(records))
	decryptWarnCount := 0
	for _, record := range records {
		decryptedPassword, normalizedStored, err := utils.ResolveBackupPassword(a.Config.SecretKey, record.BackupPassword)
		if err != nil {
			decryptWarnCount++
			decryptedPassword = ""
		}
		displayPassword := decryptedPassword
		if !revealPasswords {
			displayPassword = maskBackupPassword(decryptedPassword)
		}
		if normalizedStored != "" && normalizedStored != record.BackupPassword {
			_ = a.DB.Model(&models.BackupNotification{}).
				Where("id = ?", record.ID).
				Update("backup_password", normalizedStored).Error
		}
		items = append(items, adminBackupNotificationItem{
			ID:             record.ID,
			BackupFilePath: record.BackupFilePath,
			BackupPassword: displayPassword,
			RecipientEmail: record.RecipientEmail,
			SentAt:         record.SentAt,
			CreatedAt:      record.CreatedAt,
		})
	}

	c.HTML(http.StatusOK, "admin/backup_notifications.html", gin.H{
		"Title":     "备份通知",
		"Items":     items,
		"Msg":       strings.TrimSpace(c.Query("msg")),
		"Error":     strings.TrimSpace(c.Query("error")),
		"Retention": a.ConfigCenter.GetInt("BACKUP_RETENTION_DAYS", 30),
		"WarnCount": decryptWarnCount,
		"Reveal":    revealPasswords,
	})
}

func (a *AppContext) adminRunBackupNow(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Minute)
	defer cancel()

	if err := scheduler.RunBackupJob(ctx, a.DB, a.Config, a.ConfigCenter); err != nil {
		if errors.Is(err, scheduler.ErrBackupJobAlreadyRunning) {
			c.Redirect(http.StatusFound, "/admin/backup-notifications?error=已有备份任务正在执行，请稍后重试")
			return
		}
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+url.QueryEscape(err.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/admin/backup-notifications?msg=手动备份已执行")
}

func (a *AppContext) adminRestoreBackupUpload(c *gin.Context) {
	file, err := c.FormFile("backup_file")
	if err != nil || file == nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=请选择备份文件")
		return
	}

	baseName := sanitizeUploadedFileName(file.Filename)
	if baseName == "" {
		baseName = "backup.sql"
	}
	ext := strings.ToLower(filepath.Ext(baseName))
	if ext != ".enc" && ext != ".zip" && ext != ".sql" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=仅支持 .enc/.zip/.sql 备份文件")
		return
	}

	password := strings.TrimSpace(c.PostForm("password"))
	clean := true

	dir := filepath.Join("backups", "imports", time.Now().Format("20060102"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+url.QueryEscape("创建上传目录失败："+err.Error()))
		return
	}
	storedName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), baseName)
	storedPath := filepath.Join(dir, storedName)
	if err := c.SaveUploadedFile(file, storedPath); err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+url.QueryEscape("保存备份文件失败："+err.Error()))
		return
	}
	defer os.Remove(storedPath)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Minute)
	defer cancel()

	if err := utils.RestoreDatabaseBackup(ctx, a.Config, storedPath, password, clean); err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+url.QueryEscape("恢复失败："+err.Error()))
		return
	}

	c.Redirect(http.StatusFound, "/admin/backup-notifications?msg=备份恢复完成")
}

func (a *AppContext) adminDownloadBackupFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=无效ID")
		return
	}

	var record models.BackupNotification
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=记录不存在")
		return
	}

	filePath := filepath.Clean(strings.TrimSpace(record.BackupFilePath))
	if filePath == "" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件路径为空")
		return
	}
	if _, statErr := os.Stat(filePath); statErr != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件不存在")
		return
	}

	c.FileAttachment(filePath, filepath.Base(filePath))
}

func (a *AppContext) adminDecryptDownloadBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=无效ID")
		return
	}
	password := strings.TrimSpace(c.PostForm("password"))
	if password == "" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=解密密码不能为空")
		return
	}

	var record models.BackupNotification
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=记录不存在")
		return
	}

	encPath := filepath.Clean(strings.TrimSpace(record.BackupFilePath))
	if encPath == "" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件路径为空")
		return
	}
	if _, statErr := os.Stat(encPath); statErr != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件不存在")
		return
	}

	zipPath, err := utils.DecryptBackupBase64FileToTemp(encPath, password)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=解密失败，请检查密码")
		return
	}
	defer os.Remove(zipPath)

	c.FileAttachment(zipPath, filepath.Base(zipPath))
}

func (a *AppContext) adminDecryptDownloadBackupAuto(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=无效ID")
		return
	}

	var record models.BackupNotification
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=记录不存在")
		return
	}

	password, normalizedStored, err := utils.ResolveBackupPassword(a.Config.SecretKey, record.BackupPassword)
	if err != nil || strings.TrimSpace(password) == "" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=自动解密失败，备份密码不可用")
		return
	}
	if normalizedStored != "" && normalizedStored != record.BackupPassword {
		_ = a.DB.Model(&models.BackupNotification{}).
			Where("id = ?", record.ID).
			Update("backup_password", normalizedStored).Error
	}

	encPath := filepath.Clean(strings.TrimSpace(record.BackupFilePath))
	if encPath == "" {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件路径为空")
		return
	}
	if _, statErr := os.Stat(encPath); statErr != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=备份文件不存在")
		return
	}

	zipPath, err := utils.DecryptBackupBase64FileToTemp(encPath, password)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=自动解密失败，请检查记录密码")
		return
	}
	defer os.Remove(zipPath)

	c.FileAttachment(zipPath, filepath.Base(zipPath))
}

func (a *AppContext) adminCleanupAuditLogs(c *gin.Context) {
	days := a.ConfigCenter.GetInt("AUDIT_RETENTION_DAYS", 90)
	if raw := strings.TrimSpace(c.PostForm("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days <= 0 {
		c.Redirect(http.StatusFound, "/admin/audit-logs?error=保留天数必须大于0")
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	result := a.DB.Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	if result.Error != nil {
		c.Redirect(http.StatusFound, "/admin/audit-logs?error="+result.Error.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/audit-logs?msg="+fmt.Sprintf("已清理 %d 条审计日志", result.RowsAffected))
}

func (a *AppContext) adminCleanupBackupRecords(c *gin.Context) {
	days := a.ConfigCenter.GetInt("BACKUP_RETENTION_DAYS", 30)
	if raw := strings.TrimSpace(c.PostForm("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days <= 0 {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error=保留天数必须大于0")
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var oldRecords []models.BackupNotification
	_ = a.DB.Where("created_at < ?", cutoff).Find(&oldRecords).Error

	removedFiles := int64(0)
	for _, record := range oldRecords {
		filePath := filepath.Clean(strings.TrimSpace(record.BackupFilePath))
		if filePath == "" {
			continue
		}
		if err := os.Remove(filePath); err == nil {
			removedFiles++
		}
	}

	dbResult := a.DB.Where("created_at < ?", cutoff).Delete(&models.BackupNotification{})
	if dbResult.Error != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+dbResult.Error.Error())
		return
	}

	if _, err := utils.CleanupOldBackupFiles("backups", days); err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?msg="+fmt.Sprintf("已清理 %d 条备份记录，文件清理部分失败", dbResult.RowsAffected))
		return
	}

	c.Redirect(http.StatusFound, "/admin/backup-notifications?msg="+fmt.Sprintf("已清理 %d 条记录，删除 %d 个文件", dbResult.RowsAffected, removedFiles))
}

func (a *AppContext) adminNormalizeBackupPasswords(c *gin.Context) {
	var records []models.BackupNotification
	if err := a.DB.Select("id", "backup_password").Find(&records).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/backup-notifications?error="+err.Error())
		return
	}

	updated := int64(0)
	failed := int64(0)

	for _, record := range records {
		_, normalizedStored, err := utils.ResolveBackupPassword(a.Config.SecretKey, record.BackupPassword)
		if err != nil {
			failed++
			continue
		}
		if strings.TrimSpace(normalizedStored) == "" || normalizedStored == record.BackupPassword {
			continue
		}
		if err := a.DB.Model(&models.BackupNotification{}).
			Where("id = ?", record.ID).
			Update("backup_password", normalizedStored).Error; err != nil {
			failed++
			continue
		}
		updated++
	}

	msg := fmt.Sprintf("密码规范化完成：更新 %d 条，失败 %d 条", updated, failed)
	c.Redirect(http.StatusFound, "/admin/backup-notifications?msg="+msg)
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func maskBackupPassword(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return "****"
	}
	if len(trimmed) <= 8 {
		return trimmed[:1] + "****" + trimmed[len(trimmed)-1:]
	}
	return trimmed[:2] + "****" + trimmed[len(trimmed)-2:]
}
