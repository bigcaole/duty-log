package handlers

import (
	"net/http"
	"sync"

	"duty-log-system/internal/config"
	"duty-log-system/internal/scheduler"
	"duty-log-system/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type AppContext struct {
	DB           *gorm.DB
	ConfigCenter *utils.ConfigCenter
	Config       config.AppConfig
	summaryMu    sync.RWMutex
	summaryCache map[string]utils.WeeklySummaryResult
	backupMu     sync.Mutex
	backupCron   *cron.Cron
	reportMu     sync.Mutex
	reportCron   *cron.Cron
	reminderMu   sync.Mutex
	reminderCron *cron.Cron
}

func NewAppContext(db *gorm.DB, configCenter *utils.ConfigCenter, cfg config.AppConfig) *AppContext {
	return &AppContext{
		DB:           db,
		ConfigCenter: configCenter,
		Config:       cfg,
		summaryCache: make(map[string]utils.WeeklySummaryResult),
	}
}

func (a *AppContext) SetWeeklySummary(result utils.WeeklySummaryResult) {
	a.SetSummary("week", result)
}

func (a *AppContext) SetSummary(reportType string, result utils.WeeklySummaryResult) {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	if a.summaryCache == nil {
		a.summaryCache = make(map[string]utils.WeeklySummaryResult)
	}
	a.summaryCache[normalizeSummaryCacheKey(reportType)] = result
}

func (a *AppContext) GetWeeklySummary() (utils.WeeklySummaryResult, bool) {
	return a.GetSummary("week")
}

func (a *AppContext) GetSummary(reportType string) (utils.WeeklySummaryResult, bool) {
	a.summaryMu.RLock()
	defer a.summaryMu.RUnlock()
	if a.summaryCache == nil {
		return utils.WeeklySummaryResult{}, false
	}
	result, ok := a.summaryCache[normalizeSummaryCacheKey(reportType)]
	return result, ok
}

func (a *AppContext) SetBackupScheduler(worker *cron.Cron) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	if a.backupCron != nil && a.backupCron != worker {
		a.backupCron.Stop()
	}
	a.backupCron = worker
}

func (a *AppContext) ReloadBackupScheduler() error {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()

	if a.backupCron != nil {
		a.backupCron.Stop()
		a.backupCron = nil
	}

	worker, err := scheduler.StartBackupScheduler(a.DB, a.Config, a.ConfigCenter)
	if err != nil {
		return err
	}
	a.backupCron = worker
	return nil
}

func (a *AppContext) StopBackupScheduler() {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	if a.backupCron != nil {
		a.backupCron.Stop()
		a.backupCron = nil
	}
}

func (a *AppContext) SetReportScheduler(worker *cron.Cron) {
	a.reportMu.Lock()
	defer a.reportMu.Unlock()
	if a.reportCron != nil && a.reportCron != worker {
		a.reportCron.Stop()
	}
	a.reportCron = worker
}

func (a *AppContext) ReloadReportScheduler() error {
	a.reportMu.Lock()
	defer a.reportMu.Unlock()

	if a.reportCron != nil {
		a.reportCron.Stop()
		a.reportCron = nil
	}

	worker, err := scheduler.StartWeeklyReportScheduler(a.DB, a.ConfigCenter, a.SetWeeklySummary)
	if err != nil {
		return err
	}
	a.reportCron = worker
	return nil
}

func (a *AppContext) StopReportScheduler() {
	a.reportMu.Lock()
	defer a.reportMu.Unlock()
	if a.reportCron != nil {
		a.reportCron.Stop()
		a.reportCron = nil
	}
}

func (a *AppContext) SetReminderScheduler(worker *cron.Cron) {
	a.reminderMu.Lock()
	defer a.reminderMu.Unlock()
	if a.reminderCron != nil && a.reminderCron != worker {
		a.reminderCron.Stop()
	}
	a.reminderCron = worker
}

func (a *AppContext) ReloadReminderScheduler() error {
	a.reminderMu.Lock()
	defer a.reminderMu.Unlock()

	if a.reminderCron != nil {
		a.reminderCron.Stop()
		a.reminderCron = nil
	}

	worker, err := scheduler.StartReminderScheduler(a.DB, a.ConfigCenter)
	if err != nil {
		return err
	}
	a.reminderCron = worker
	return nil
}

func (a *AppContext) StopReminderScheduler() {
	a.reminderMu.Lock()
	defer a.reminderMu.Unlock()
	if a.reminderCron != nil {
		a.reminderCron.Stop()
		a.reminderCron = nil
	}
}

func renderComingSoon(c *gin.Context, title, path string) {
	c.HTML(http.StatusOK, "coming_soon.html", gin.H{
		"Title":   title,
		"Path":    path,
		"Message": "该模块将在下一部分实现完整 CRUD 与业务逻辑。",
	})
}

func normalizeSummaryCacheKey(reportType string) string {
	switch reportType {
	case "month":
		return "month"
	case "halfyear":
		return "halfyear"
	case "year":
		return "year"
	default:
		return "week"
	}
}
