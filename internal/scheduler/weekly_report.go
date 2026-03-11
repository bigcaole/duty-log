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
	FeishuAttempted bool
	FeishuSent      bool
}

func StartWeeklyReportScheduler(db *gorm.DB, configCenter *utils.ConfigCenter, onGenerated func(utils.WeeklySummaryResult)) (*cron.Cron, error) {
	if db == nil || configCenter == nil {
		return nil, fmt.Errorf("invalid weekly report scheduler dependencies")
	}

	enabled := configCenter.GetBool("REPORT_FEISHU_ENABLED", false)
	if !enabled {
		log.Printf("weekly report scheduler disabled")
		return nil, nil
	}

	schedule := "0 17 * * *"

	worker := cron.New(cron.WithLocation(time.Local))
	_, err := worker.AddFunc(schedule, func() {
		if !tryStartWeeklyReportJob() {
			log.Printf("weekly report job skipped: %v", ErrWeeklyReportJobAlreadyRunning)
			return
		}
		defer finishWeeklyReportJob()

		periods := reportPeriodsForNow(time.Now())
		if len(periods) == 0 {
			return
		}
		for _, period := range periods {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			result, runErr := RunPeriodicReportJob(ctx, db, configCenter, period, WeeklyReportRunOptions{}, onGenerated)
			cancel()
			if runErr != nil {
				log.Printf("periodic report job failed (%s): %v", period, runErr)
				continue
			}
			log.Printf(
				"periodic report job done: type=%s period=%s~%s feishu_attempted=%t feishu_sent=%t",
				period,
				result.Summary.PeriodStart.Format("2006-01-02"),
				result.Summary.PeriodEnd.Format("2006-01-02"),
				result.FeishuAttempted,
				result.FeishuSent,
			)
		}
	})
	if err != nil {
		return nil, err
	}

	worker.Start()
	log.Printf("weekly report scheduler started with cron: %s", schedule)
	return worker, nil
}

func RunPeriodicReportJob(
	ctx context.Context,
	db *gorm.DB,
	configCenter *utils.ConfigCenter,
	reportType string,
	options WeeklyReportRunOptions,
	onGenerated func(utils.WeeklySummaryResult),
) (WeeklyReportRunResult, error) {
	if db == nil || configCenter == nil {
		return WeeklyReportRunResult{}, fmt.Errorf("invalid report dependencies")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	summary, err := utils.GeneratePeriodicSummary(ctx, db, configCenter, time.Now(), reportType)
	if err != nil {
		return WeeklyReportRunResult{}, err
	}
	if onGenerated != nil {
		onGenerated(summary)
	}

	result := WeeklyReportRunResult{Summary: summary}

	feishuEnabled := configCenter.GetBool("REPORT_FEISHU_ENABLED", false)
	if options.ForceNotify {
		feishuEnabled = true
	}

	if feishuEnabled {
		result.FeishuAttempted = true
		if sendErr := sendPeriodicSummaryFeishu(configCenter, summary); sendErr != nil {
			return result, fmt.Errorf("飞书推送失败: %w", sendErr)
		}
		result.FeishuSent = true
	}
	return result, nil
}

func reportPeriodsForNow(now time.Time) []string {
	if now.IsZero() {
		return nil
	}
	periods := make([]string, 0, 4)
	if now.Weekday() == time.Sunday {
		periods = append(periods, "week")
	}
	if isLastDayOfMonth(now) {
		periods = append(periods, "month")
	}
	if now.Month() == time.June && (now.Day() == 25 || now.Day() == 30) {
		periods = append(periods, "halfyear")
	}
	if now.Month() == time.December && (now.Day() == 25 || now.Day() == 31) {
		periods = append(periods, "year")
	}
	return periods
}

func isLastDayOfMonth(t time.Time) bool {
	next := t.AddDate(0, 0, 1)
	return next.Month() != t.Month()
}

func sendPeriodicSummaryFeishu(configCenter *utils.ConfigCenter, summary utils.WeeklySummaryResult) error {
	webhook := strings.TrimSpace(configCenter.Get("FEISHU_WEBHOOK_URL", ""))
	if webhook == "" {
		return fmt.Errorf("FEISHU_WEBHOOK_URL 未配置")
	}

	title := fmt.Sprintf("%s %s~%s", summary.ReportTypeLabel, summary.PeriodStart.Format("2006-01-02"), summary.PeriodEnd.Format("2006-01-02"))
	content := fmt.Sprintf(
		"生成时间：%s\n%s",
		summary.GeneratedAt.Format("2006-01-02 15:04:05"),
		strings.TrimSpace(summary.Summary),
	)

	chunks := splitFeishuContent(content, 1500)
	for i, chunk := range chunks {
		partTitle := title
		if len(chunks) > 1 {
			partTitle = fmt.Sprintf("%s（%d/%d）", title, i+1, len(chunks))
		}
		if err := utils.SendFeishuText(webhook, partTitle, chunk); err != nil {
			return err
		}
	}
	return nil
}

func splitFeishuContent(content string, maxRunes int) []string {
	text := strings.TrimSpace(content)
	if text == "" {
		return []string{"-"}
	}
	if maxRunes <= 0 {
		maxRunes = 1500
	}
	paragraphs := strings.Split(text, "\n")
	chunks := make([]string, 0, 4)
	var current strings.Builder
	currentLen := 0

	flush := func() {
		if currentLen == 0 {
			return
		}
		chunks = append(chunks, strings.TrimSpace(current.String()))
		current.Reset()
		currentLen = 0
	}

	for _, para := range paragraphs {
		segment := strings.TrimSpace(para)
		if segment == "" {
			continue
		}
		segmentLen := len([]rune(segment)) + 1
		if currentLen+segmentLen > maxRunes {
			flush()
		}
		if currentLen > 0 {
			current.WriteString("\n")
			currentLen++
		}
		current.WriteString(segment)
		currentLen += len([]rune(segment))
	}
	flush()
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
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
