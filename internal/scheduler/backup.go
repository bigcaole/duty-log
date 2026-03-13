package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"duty-log-system/internal/config"
	"duty-log-system/internal/models"
	"duty-log-system/pkg/utils"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var ErrBackupJobAlreadyRunning = errors.New("backup job is already running")

var backupRunGuard = struct {
	mu      sync.Mutex
	running bool
}{}

func StartBackupScheduler(db *gorm.DB, appConfig config.AppConfig, configCenter *utils.ConfigCenter) (*cron.Cron, error) {
	if db == nil || configCenter == nil {
		return nil, fmt.Errorf("invalid backup scheduler dependencies")
	}

	enabled := configCenter.GetBool("BACKUP_ENABLED", false)
	if !enabled {
		log.Printf("backup scheduler disabled")
		return nil, nil
	}

	schedule := strings.TrimSpace(configCenter.Get("BACKUP_SCHEDULE", ""))
	if schedule == "" {
		schedule = "0 18 * * 0"
	}

	worker := cron.New(cron.WithLocation(time.Local))
	_, err := worker.AddFunc(schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if runErr := RunBackupJob(ctx, db, appConfig, configCenter); runErr != nil {
			if errors.Is(runErr, ErrBackupJobAlreadyRunning) {
				log.Printf("backup job skipped: %v", runErr)
				return
			}
			log.Printf("backup job failed: %v", runErr)
		}
	})
	if err != nil {
		return nil, err
	}

	worker.Start()
	log.Printf("backup scheduler started with cron: %s", schedule)
	return worker, nil
}

func RunBackupJob(ctx context.Context, db *gorm.DB, appConfig config.AppConfig, configCenter *utils.ConfigCenter) error {
	if !tryStartBackupJob() {
		return ErrBackupJobAlreadyRunning
	}
	defer finishBackupJob()

	if ctx == nil {
		ctx = context.Background()
	}
	result, err := utils.CreateDatabaseBackup(ctx, appConfig, "backups")
	if err != nil {
		return err
	}
	encryptedBackupPassword, encErr := utils.EncryptBackupPassword(appConfig.SecretKey, result.Password)
	if encErr != nil {
		return fmt.Errorf("encrypt backup password failed: %w", encErr)
	}

	backupFileName := filepath.Base(result.FilePath)
	backupData, readErr := os.ReadFile(result.FilePath)
	if readErr != nil {
		return readErr
	}

	nextcloudStatus := "未配置"
	nextcloudURL := strings.TrimSpace(configCenter.Get("NEXTCLOUD_URL", ""))
	nextcloudUser := strings.TrimSpace(configCenter.Get("NEXTCLOUD_USERNAME", ""))
	nextcloudPass := strings.TrimSpace(configCenter.Get("NEXTCLOUD_PASSWORD", ""))
	nextcloudPath := strings.TrimSpace(configCenter.Get("NEXTCLOUD_PATH", ""))
	nextcloudInsecure := configCenter.GetBool("NEXTCLOUD_TLS_INSECURE", false)
	if nextcloudURL != "" && nextcloudUser != "" && nextcloudPass != "" {
		err := utils.UploadToNextcloud(ctx, utils.NextcloudConfig{
			BaseURL:            nextcloudURL,
			Username:           nextcloudUser,
			Password:           nextcloudPass,
			RemotePath:         nextcloudPath,
			InsecureSkipVerify: nextcloudInsecure,
		}, result.FilePath)
		if err != nil {
			nextcloudStatus = "失败"
			log.Printf("nextcloud upload failed: %v", err)
		} else {
			nextcloudStatus = "成功"
		}
	}

	var sentAt *time.Time
	recipientEmail := strings.TrimSpace(configCenter.Get("BACKUP_EMAIL", ""))
	if recipientEmail != "" {
		smtpConfig := utils.SMTPConfig{
			Server:        configCenter.Get("MAIL_SERVER", "smtp.gmail.com"),
			Port:          configCenter.GetInt("MAIL_PORT", 587),
			UseTLS:        configCenter.GetBool("MAIL_USE_TLS", true),
			Username:      configCenter.Get("MAIL_USERNAME", ""),
			Password:      configCenter.Get("MAIL_PASSWORD", ""),
			DefaultSender: configCenter.Get("MAIL_DEFAULT_SENDER", ""),
		}

		subject := "数据库备份 - " + time.Now().Format("2006-01-02 15:04:05")
		body := fmt.Sprintf("备份文件已生成。\n文件: %s\n解密密码: %s", backupFileName, result.Password)
		attachment := utils.NewAttachmentFromFile(backupFileName, backupData)
		if err := utils.SendEmail(smtpConfig, []string{recipientEmail}, subject, body, "", []utils.EmailAttachment{attachment}); err == nil {
			now := time.Now()
			sentAt = &now
		} else {
			log.Printf("send backup email failed: %v", err)
		}
	}

	notify := models.BackupNotification{
		BackupFilePath: result.FilePath,
		BackupPassword: encryptedBackupPassword,
		RecipientEmail: recipientEmail,
		SentAt:         sentAt,
	}
	if err := db.Create(&notify).Error; err != nil {
		log.Printf("save backup notification failed: %v", err)
	}

	webhook := strings.TrimSpace(configCenter.Get("FEISHU_WEBHOOK_URL", ""))
	if webhook != "" {
		message := fmt.Sprintf("备份文件: %s\n发送邮箱: %s\nNextcloud: %s", backupFileName, recipientEmail, nextcloudStatus)
		_ = utils.SendFeishuText(webhook, "数据库备份通知", message)
	}

	retentionDays := configCenter.GetInt("BACKUP_RETENTION_DAYS", 30)
	if retentionDays > 0 {
		if removed, cleanupErr := utils.CleanupOldBackupFiles("backups", retentionDays); cleanupErr != nil {
			log.Printf("cleanup old backup files failed: %v", cleanupErr)
		} else if len(removed) > 0 {
			log.Printf("cleanup old backup files: removed=%d", len(removed))
		}

		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		if err := db.Where("created_at < ?", cutoff).Delete(&models.BackupNotification{}).Error; err != nil {
			log.Printf("cleanup old backup notifications failed: %v", err)
		}
	}

	return nil
}

func tryStartBackupJob() bool {
	backupRunGuard.mu.Lock()
	defer backupRunGuard.mu.Unlock()
	if backupRunGuard.running {
		return false
	}
	backupRunGuard.running = true
	return true
}

func finishBackupJob() {
	backupRunGuard.mu.Lock()
	backupRunGuard.running = false
	backupRunGuard.mu.Unlock()
}
