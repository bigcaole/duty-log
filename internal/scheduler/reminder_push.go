package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"duty-log-system/internal/models"
	"duty-log-system/pkg/utils"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var ErrReminderJobAlreadyRunning = errors.New("reminder job is already running")

var reminderRunGuard = struct {
	mu      sync.Mutex
	running bool
}{}

func StartReminderScheduler(db *gorm.DB, configCenter *utils.ConfigCenter) (*cron.Cron, error) {
	if db == nil || configCenter == nil {
		return nil, fmt.Errorf("invalid reminder scheduler dependencies")
	}

	enabled := configCenter.GetBool("REMINDER_FEISHU_ENABLED", false)
	if !enabled {
		log.Printf("reminder scheduler disabled")
		return nil, nil
	}

	worker := cron.New(cron.WithLocation(time.Local))
	_, err := worker.AddFunc("*/1 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		count, runErr := RunReminderPushJob(ctx, db, configCenter)
		if runErr != nil {
			if errors.Is(runErr, ErrReminderJobAlreadyRunning) {
				log.Printf("reminder job skipped: %v", runErr)
				return
			}
			log.Printf("reminder job failed: %v", runErr)
			return
		}
		if count > 0 {
			log.Printf("reminder job done: sent=%d", count)
		}
	})
	if err != nil {
		return nil, err
	}

	worker.Start()
	log.Printf("reminder scheduler started with cron: */1 * * * *")
	return worker, nil
}

func RunReminderPushJob(ctx context.Context, db *gorm.DB, configCenter *utils.ConfigCenter) (int, error) {
	if db == nil || configCenter == nil {
		return 0, fmt.Errorf("invalid reminder dependencies")
	}
	if !tryStartReminderJob() {
		return 0, ErrReminderJobAlreadyRunning
	}
	defer finishReminderJob()

	if ctx == nil {
		ctx = context.Background()
	}

	webhook := strings.TrimSpace(configCenter.Get("FEISHU_WEBHOOK_URL", ""))
	if webhook == "" {
		return 0, fmt.Errorf("FEISHU_WEBHOOK_URL 未配置")
	}

	var rows []models.Reminder
	if err := db.Where("is_completed = ? AND notified_at IS NULL", false).
		Order("end_date asc").
		Limit(300).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	now := time.Now()
	sent := 0
	for _, row := range rows {
		if now.Before(reminderTriggerTime(row)) {
			continue
		}
		content := strings.TrimSpace(row.Title)
		if strings.TrimSpace(row.Content) != "" {
			content = content + "\n" + strings.TrimSpace(row.Content)
		}
		if content == "" {
			content = "提醒事项"
		}
		if err := utils.SendFeishuText(webhook, "事项提醒", content); err != nil {
			log.Printf("reminder feishu send failed: %v", err)
			continue
		}
		sent++
		nowCopy := now
		_ = db.Model(&models.Reminder{}).
			Where("id = ?", row.ID).
			Update("notified_at", &nowCopy).Error
	}
	return sent, nil
}

func reminderTriggerTime(record models.Reminder) time.Time {
	deadline := reminderDeadlineTime(record)
	days := record.RemindDaysBefore
	if days < 0 {
		days = 0
	}
	return deadline.AddDate(0, 0, -days)
}

func reminderDeadlineTime(record models.Reminder) time.Time {
	end := time.Date(record.EndDate.Year(), record.EndDate.Month(), record.EndDate.Day(), 0, 0, 0, 0, record.EndDate.Location())
	hour, minute := parseReminderClock(record.RemindTime)
	return time.Date(end.Year(), end.Month(), end.Day(), hour, minute, 0, 0, end.Location())
}

func parseReminderClock(raw string) (int, int) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 9, 0
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		hour = 9
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		minute = 0
	}
	return hour, minute
}

func tryStartReminderJob() bool {
	reminderRunGuard.mu.Lock()
	defer reminderRunGuard.mu.Unlock()
	if reminderRunGuard.running {
		return false
	}
	reminderRunGuard.running = true
	return true
}

func finishReminderJob() {
	reminderRunGuard.mu.Lock()
	reminderRunGuard.running = false
	reminderRunGuard.mu.Unlock()
}
