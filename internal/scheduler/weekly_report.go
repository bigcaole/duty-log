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

	schedule := "*/1 * * * *"

	worker := cron.New(cron.WithLocation(time.Local))
	_, err := worker.AddFunc(schedule, func() {
		if !tryStartWeeklyReportJob() {
			log.Printf("weekly report job skipped: %v", ErrWeeklyReportJobAlreadyRunning)
			return
		}
		defer finishWeeklyReportJob()

		periods := reportPeriodsForNow(time.Now(), configCenter)
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

func reportPeriodsForNow(now time.Time, configCenter *utils.ConfigCenter) []string {
	if now.IsZero() {
		return nil
	}
	schedule := loadReportSchedule(configCenter)
	if !sameReportClock(now, schedule.WeeklyHour, schedule.WeeklyMinute) &&
		!sameReportClock(now, schedule.MonthlyHour, schedule.MonthlyMinute) &&
		!sameReportClock(now, schedule.HalfYearHour, schedule.HalfYearMinute) &&
		!sameReportClock(now, schedule.YearHour, schedule.YearMinute) {
		return nil
	}
	periods := make([]string, 0, 4)
	if sameReportClock(now, schedule.WeeklyHour, schedule.WeeklyMinute) && int(now.Weekday()) == schedule.WeeklyWeekday {
		periods = append(periods, "week")
	}
	if sameReportClock(now, schedule.MonthlyHour, schedule.MonthlyMinute) && isMonthlyReportDay(now, schedule.MonthlyDay) {
		periods = append(periods, "month")
	}
	if sameReportClock(now, schedule.HalfYearHour, schedule.HalfYearMinute) &&
		now.Month() == time.June &&
		(now.Day() == schedule.HalfYearDay1 || now.Day() == schedule.HalfYearDay2) {
		periods = append(periods, "halfyear")
	}
	if sameReportClock(now, schedule.YearHour, schedule.YearMinute) &&
		now.Month() == time.December &&
		(now.Day() == schedule.YearDay1 || now.Day() == schedule.YearDay2) {
		periods = append(periods, "year")
	}
	return periods
}

func isLastDayOfMonth(t time.Time) bool {
	next := t.AddDate(0, 0, 1)
	return next.Month() != t.Month()
}

type reportScheduleConfig struct {
	WeeklyWeekday  int
	WeeklyHour     int
	WeeklyMinute   int
	MonthlyDay     string
	MonthlyHour    int
	MonthlyMinute  int
	HalfYearDay1   int
	HalfYearDay2   int
	HalfYearHour   int
	HalfYearMinute int
	YearDay1       int
	YearDay2       int
	YearHour       int
	YearMinute     int
}

func loadReportSchedule(configCenter *utils.ConfigCenter) reportScheduleConfig {
	if configCenter == nil {
		return reportScheduleConfig{
			WeeklyWeekday:  0,
			WeeklyHour:     17,
			WeeklyMinute:   0,
			MonthlyDay:     "last",
			MonthlyHour:    17,
			MonthlyMinute:  0,
			HalfYearDay1:   25,
			HalfYearDay2:   30,
			HalfYearHour:   17,
			HalfYearMinute: 0,
			YearDay1:       25,
			YearDay2:       31,
			YearHour:       17,
			YearMinute:     0,
		}
	}
	weeklyWeekday := configCenter.GetInt("REPORT_WEEKLY_WEEKDAY", 0)
	weeklyHour, weeklyMinute := parseReportClock(configCenter.Get("REPORT_WEEKLY_TIME", "17:00"), 17, 0)
	monthlyDay := strings.TrimSpace(configCenter.Get("REPORT_MONTHLY_DAY", "last"))
	monthlyHour, monthlyMinute := parseReportClock(configCenter.Get("REPORT_MONTHLY_TIME", "17:00"), 17, 0)
	halfDay1 := configCenter.GetInt("REPORT_HALFYEAR_DAY1", 25)
	halfDay2 := configCenter.GetInt("REPORT_HALFYEAR_DAY2", 30)
	halfHour, halfMinute := parseReportClock(configCenter.Get("REPORT_HALFYEAR_TIME", "17:00"), 17, 0)
	yearDay1 := configCenter.GetInt("REPORT_YEAR_DAY1", 25)
	yearDay2 := configCenter.GetInt("REPORT_YEAR_DAY2", 31)
	yearHour, yearMinute := parseReportClock(configCenter.Get("REPORT_YEAR_TIME", "17:00"), 17, 0)

	if weeklyWeekday < 0 || weeklyWeekday > 6 {
		weeklyWeekday = 0
	}
	if monthlyDay == "" {
		monthlyDay = "last"
	}
	if halfDay1 < 1 || halfDay1 > 31 {
		halfDay1 = 25
	}
	if halfDay2 < 1 || halfDay2 > 31 {
		halfDay2 = 30
	}
	if yearDay1 < 1 || yearDay1 > 31 {
		yearDay1 = 25
	}
	if yearDay2 < 1 || yearDay2 > 31 {
		yearDay2 = 31
	}

	return reportScheduleConfig{
		WeeklyWeekday:  weeklyWeekday,
		WeeklyHour:     weeklyHour,
		WeeklyMinute:   weeklyMinute,
		MonthlyDay:     monthlyDay,
		MonthlyHour:    monthlyHour,
		MonthlyMinute:  monthlyMinute,
		HalfYearDay1:   halfDay1,
		HalfYearDay2:   halfDay2,
		HalfYearHour:   halfHour,
		HalfYearMinute: halfMinute,
		YearDay1:       yearDay1,
		YearDay2:       yearDay2,
		YearHour:       yearHour,
		YearMinute:     yearMinute,
	}
}

func parseReportClock(raw string, fallbackHour, fallbackMinute int) (int, int) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return fallbackHour, fallbackMinute
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		hour = fallbackHour
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		minute = fallbackMinute
	}
	return hour, minute
}

func sameReportClock(now time.Time, hour, minute int) bool {
	return now.Hour() == hour && now.Minute() == minute
}

func isMonthlyReportDay(now time.Time, day string) bool {
	if strings.EqualFold(strings.TrimSpace(day), "last") {
		return isLastDayOfMonth(now)
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(day))
	if err != nil || parsed < 1 || parsed > 31 {
		return isLastDayOfMonth(now)
	}
	return now.Day() == parsed
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
