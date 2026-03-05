package handlers

import (
	"net/http"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

func registerMainRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/dashboard", app.dashboard)
}

func (a *AppContext) dashboard(c *gin.Context) {
	user, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var dutyLogCount int64
	var idcDutyCount int64
	var ticketCount int64
	var workTicketCount int64
	var faultCount int64
	_ = applyUserScope(a.DB.Model(&models.DutyLog{}), user.IsAdmin, user.ID, "user_id").Count(&dutyLogCount).Error
	_ = applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), user.IsAdmin, user.ID, "user_id").Count(&idcDutyCount).Error
	_ = applyUserScope(a.DB.Model(&models.Ticket{}), user.IsAdmin, user.ID, "user_id").Count(&ticketCount).Error
	_ = applyUserScope(a.DB.Model(&models.WorkTicket{}), user.IsAdmin, user.ID, "user_id").Count(&workTicketCount).Error
	_ = applyUserScope(a.DB.Model(&models.FaultRecord{}), user.IsAdmin, user.ID, "user_id").Count(&faultCount).Error

	c.HTML(http.StatusOK, "dashboard.html", map[string]any{
		"Title":           "值班管理系统",
		"Username":        user.Username,
		"IsAdmin":         user.IsAdmin,
		"Has2FA":          user.OTPSecret != "",
		"BackupCron":      a.ConfigCenter.Get("BACKUP_SCHEDULE", "0 2 * * *"),
		"ScopeLabel":      dashboardScopeLabel(user.IsAdmin),
		"DutyLogCount":    dutyLogCount,
		"IDCDutyCount":    idcDutyCount,
		"TicketCount":     ticketCount,
		"WorkTicketCount": workTicketCount,
		"FaultCount":      faultCount,
	})
}

func dashboardScopeLabel(isAdmin bool) string {
	if isAdmin {
		return "全局数据"
	}
	return "我的数据"
}
