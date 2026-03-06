package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"duty-log-system/pkg/utils"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var ErrWeeklyReportJobAlreadyRunning = errors.New("weekly report job is already running")

var weeklyReportRunGuard = struct {
	mu      sync.Mutex
	running bool
}{}

type WeeklyReportRunOptions struct {
	ForceNotify bool
}

type WeeklyReportRunResult struct {
	Summary         utils.WeeklySummaryResult
	EmailAttempted  bool
	EmailSent       bool
	FeishuAttempted bool
	FeishuSent      bool
}

func StartWeeklyReportScheduler(db *gorm.DB, configCenter *utils.ConfigCenter, onGenerated func(utils.WeeklySummaryResult)) (*cron.Cron, error) {
	if db == nil || configCenter == nil {
		return nil, fmt.Errorf("invalid weekly report scheduler dependencies")
	}

	enabled := configCenter.GetBool("WEEKLY_REPORT_ENABLED", false)
	if !enabled {
		log.Printf("weekly report scheduler disabled")
		return nil, nil
	}

	schedule := strings.TrimSpace(configCenter.Get("WEEKLY_REPORT_SCHEDULE", "0 9 * * 1"))
	if schedule == "" {
		schedule = "0 9 * * 1"
	}
	if _, err := cron.ParseStandard(schedule); err != nil {
		return nil, fmt.Errorf("invalid weekly report cron: %w", err)
	}

	worker := cron.New(cron.WithLocation(time.Local))
	_, err := worker.AddFunc(schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, runErr := RunWeeklyReportJob(ctx, db, configCenter, WeeklyReportRunOptions{}, onGenerated)
		if runErr != nil {
			if errors.Is(runErr, ErrWeeklyReportJobAlreadyRunning) {
				log.Printf("weekly report job skipped: %v", runErr)
				return
			}
			log.Printf("weekly report job failed: %v", runErr)
			return
		}
		log.Printf(
			"weekly report job done: period=%s~%s email_attempted=%t email_sent=%t feishu_attempted=%t feishu_sent=%t",
			result.Summary.PeriodStart.Format("2006-01-02"),
			result.Summary.PeriodEnd.Format("2006-01-02"),
			result.EmailAttempted,
			result.EmailSent,
			result.FeishuAttempted,
			result.FeishuSent,
		)
	})
	if err != nil {
		return nil, err
	}

	worker.Start()
	log.Printf("weekly report scheduler started with cron: %s", schedule)
	return worker, nil
}

func RunWeeklyReportJob(
	ctx context.Context,
	db *gorm.DB,
	configCenter *utils.ConfigCenter,
	options WeeklyReportRunOptions,
	onGenerated func(utils.WeeklySummaryResult),
) (WeeklyReportRunResult, error) {
	if db == nil || configCenter == nil {
		return WeeklyReportRunResult{}, fmt.Errorf("invalid weekly report dependencies")
	}
	if !tryStartWeeklyReportJob() {
		return WeeklyReportRunResult{}, ErrWeeklyReportJobAlreadyRunning
	}
	defer finishWeeklyReportJob()

	if ctx == nil {
		ctx = context.Background()
	}

	summary, err := utils.GenerateWeeklySummary(ctx, db, configCenter, time.Now())
	if err != nil {
		return WeeklyReportRunResult{}, err
	}
	if onGenerated != nil {
		onGenerated(summary)
	}

	result := WeeklyReportRunResult{Summary: summary}

	emailEnabled := configCenter.GetBool("WEEKLY_REPORT_EMAIL_ENABLED", false)
	feishuEnabled := configCenter.GetBool("WEEKLY_REPORT_FEISHU_ENABLED", false)
	if options.ForceNotify {
		emailEnabled = true
		feishuEnabled = true
	}

	errMessages := make([]string, 0, 2)
	if emailEnabled {
		result.EmailAttempted = true
		if sendErr := sendWeeklySummaryEmail(configCenter, summary); sendErr != nil {
			errMessages = append(errMessages, "邮件推送失败: "+sendErr.Error())
		} else {
			result.EmailSent = true
		}
	}

	if feishuEnabled {
		result.FeishuAttempted = true
		if sendErr := sendWeeklySummaryFeishu(configCenter, summary); sendErr != nil {
			errMessages = append(errMessages, "飞书推送失败: "+sendErr.Error())
		} else {
			result.FeishuSent = true
		}
	}

	if len(errMessages) > 0 {
		return result, fmt.Errorf(strings.Join(errMessages, "；"))
	}
	return result, nil
}

func sendWeeklySummaryEmail(configCenter *utils.ConfigCenter, summary utils.WeeklySummaryResult) error {
	recipients := parseCSVEmails(configCenter.Get("WEEKLY_REPORT_EMAIL_TO", ""))
	if len(recipients) == 0 {
		return fmt.Errorf("WEEKLY_REPORT_EMAIL_TO 未配置")
	}

	smtpConfig := utils.SMTPConfig{
		Server:        configCenter.Get("MAIL_SERVER", "smtp.gmail.com"),
		Port:          configCenter.GetInt("MAIL_PORT", 587),
		UseTLS:        configCenter.GetBool("MAIL_USE_TLS", true),
		Username:      configCenter.Get("MAIL_USERNAME", ""),
		Password:      configCenter.Get("MAIL_PASSWORD", ""),
		DefaultSender: configCenter.Get("MAIL_DEFAULT_SENDER", ""),
	}

	subject := fmt.Sprintf("周报推送 %s~%s", summary.PeriodStart.Format("2006-01-02"), summary.PeriodEnd.Format("2006-01-02"))
	body := fmt.Sprintf(
		"周报统计周期：%s ~ %s\n生成时间：%s\n值班记录：%d\n普通工单：%d\n网络工单：%d\n\n%s",
		summary.PeriodStart.Format("2006-01-02"),
		summary.PeriodEnd.Format("2006-01-02"),
		summary.GeneratedAt.Format("2006-01-02 15:04:05"),
		summary.DutyCount,
		summary.TicketCount,
		summary.WorkTicketCount,
		summary.Summary,
	)
	return utils.SendEmail(smtpConfig, recipients, subject, body, "", nil)
}

func sendWeeklySummaryFeishu(configCenter *utils.ConfigCenter, summary utils.WeeklySummaryResult) error {
	webhook := strings.TrimSpace(configCenter.Get("FEISHU_WEBHOOK_URL", ""))
	if webhook == "" {
		return fmt.Errorf("FEISHU_WEBHOOK_URL 未配置")
	}

	title := fmt.Sprintf("值班周报 %s~%s", summary.PeriodStart.Format("2006-01-02"), summary.PeriodEnd.Format("2006-01-02"))
	content := fmt.Sprintf(
		"生成时间：%s\n值班记录：%d\n普通工单：%d\n网络工单：%d\n\n%s",
		summary.GeneratedAt.Format("2006-01-02 15:04:05"),
		summary.DutyCount,
		summary.TicketCount,
		summary.WorkTicketCount,
		trimWeeklyMessage(summary.Summary, 1200),
	)
	return utils.SendFeishuText(webhook, title, content)
}

func parseCSVEmails(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func trimWeeklyMessage(raw string, max int) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "-"
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}

func tryStartWeeklyReportJob() bool {
	weeklyReportRunGuard.mu.Lock()
	defer weeklyReportRunGuard.mu.Unlock()
	if weeklyReportRunGuard.running {
		return false
	}
	weeklyReportRunGuard.running = true
	return true
}

func finishWeeklyReportJob() {
	weeklyReportRunGuard.mu.Lock()
	weeklyReportRunGuard.running = false
	weeklyReportRunGuard.mu.Unlock()
}
