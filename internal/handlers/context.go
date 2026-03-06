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
	summaryCache *utils.WeeklySummaryResult
	backupMu     sync.Mutex
	backupCron   *cron.Cron
	weeklyMu     sync.Mutex
	weeklyCron   *cron.Cron
}

func NewAppContext(db *gorm.DB, configCenter *utils.ConfigCenter, cfg config.AppConfig) *AppContext {
	return &AppContext{
		DB:           db,
		ConfigCenter: configCenter,
		Config:       cfg,
	}
}

func (a *AppContext) SetWeeklySummary(result utils.WeeklySummaryResult) {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	copied := result
	a.summaryCache = &copied
}

func (a *AppContext) GetWeeklySummary() (utils.WeeklySummaryResult, bool) {
	a.summaryMu.RLock()
	defer a.summaryMu.RUnlock()
	if a.summaryCache == nil {
		return utils.WeeklySummaryResult{}, false
	}
	return *a.summaryCache, true
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

func (a *AppContext) SetWeeklyReportScheduler(worker *cron.Cron) {
	a.weeklyMu.Lock()
	defer a.weeklyMu.Unlock()
	if a.weeklyCron != nil && a.weeklyCron != worker {
		a.weeklyCron.Stop()
	}
	a.weeklyCron = worker
}

func (a *AppContext) ReloadWeeklyReportScheduler() error {
	a.weeklyMu.Lock()
	defer a.weeklyMu.Unlock()

	if a.weeklyCron != nil {
		a.weeklyCron.Stop()
		a.weeklyCron = nil
	}

	worker, err := scheduler.StartWeeklyReportScheduler(a.DB, a.ConfigCenter, a.SetWeeklySummary)
	if err != nil {
		return err
	}
	a.weeklyCron = worker
	return nil
}

func (a *AppContext) StopWeeklyReportScheduler() {
	a.weeklyMu.Lock()
	defer a.weeklyMu.Unlock()
	if a.weeklyCron != nil {
		a.weeklyCron.Stop()
		a.weeklyCron = nil
	}
}

func renderComingSoon(c *gin.Context, title, path string) {
	c.HTML(http.StatusOK, "coming_soon.html", gin.H{
		"Title":   title,
		"Path":    path,
		"Message": "该模块将在下一部分实现完整 CRUD 与业务逻辑。",
	})
}
