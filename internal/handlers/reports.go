package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"
	"duty-log-system/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func registerReportRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/reports", app.reportPage)
	group.GET("/statistics", app.statisticsPage)
	group.GET("/export/excel", app.exportExcel)
}

func (a *AppContext) reportPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	summary := utils.WeeklySummaryResult{}
	ok := false
	if canUseGlobalWeeklySummary(currentUser.IsAdmin) {
		summary, ok = a.GetWeeklySummary()
		if !ok {
			generated, err := a.generateAndCacheWeeklySummary(c.Request.Context())
			if err == nil {
				summary = generated
				ok = true
			}
		}
	}

	c.HTML(http.StatusOK, "reports/report.html", gin.H{
		"Title":   "周报页面",
		"HasData": ok,
		"Summary": summary,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) statisticsPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	type chartItem struct {
		Name  string
		Count int64
	}

	var userCount int64
	var dutyCount int64
	var ticketCount int64
	var workTicketCount int64
	var faultCount int64
	if currentUser.IsAdmin {
		_ = a.DB.Model(&models.User{}).Count(&userCount).Error
	} else {
		userCount = 1
	}
	_ = applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&dutyCount).Error
	_ = applyUserScope(a.DB.Model(&models.Ticket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&ticketCount).Error
	_ = applyUserScope(a.DB.Model(&models.WorkTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&workTicketCount).Error
	_ = applyUserScope(a.DB.Model(&models.FaultRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&faultCount).Error

	now := time.Now()
	start7 := now.AddDate(0, 0, -7).Format("2006-01-02")
	end := now.Format("2006-01-02")

	var recentDuty int64
	var recentWork int64
	var recentFault int64
	_ = applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentDuty).Error
	_ = applyUserScope(a.DB.Model(&models.WorkTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentWork).Error
	_ = applyUserScope(a.DB.Model(&models.FaultRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentFault).Error

	charts := []chartItem{
		{Name: "IDC 值班", Count: dutyCount},
		{Name: "普通工单", Count: ticketCount},
		{Name: "网络工单", Count: workTicketCount},
		{Name: "故障记录", Count: faultCount},
	}

	c.HTML(http.StatusOK, "reports/statistics.html", gin.H{
		"Title":           "统计页面",
		"UserCount":       userCount,
		"UserCountLabel":  statisticsUserCountLabel(currentUser.IsAdmin),
		"DutyCount":       dutyCount,
		"TicketCount":     ticketCount,
		"WorkTicketCount": workTicketCount,
		"FaultCount":      faultCount,
		"RecentDuty":      recentDuty,
		"RecentWork":      recentWork,
		"RecentFault":     recentFault,
		"Charts":          charts,
	})
}

func (a *AppContext) exportExcel(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	workbook := excelize.NewFile()
	defer func() {
		_ = workbook.Close()
	}()

	_ = workbook.SetSheetName("Sheet1", "IDC Duty")
	workbook.NewSheet("Work Tickets")
	workbook.NewSheet("Fault Records")

	if err := a.exportIDCSheet(workbook, "IDC Duty", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}
	if err := a.exportWorkTicketSheet(workbook, "Work Tickets", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}
	if err := a.exportFaultSheet(workbook, "Fault Records", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}

	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("duty-log-export-%s.xlsx", time.Now().Format("20060102-150405")))
	if err := workbook.SaveAs(tempPath); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}
	defer os.Remove(tempPath)

	c.FileAttachment(tempPath, fmt.Sprintf("duty-log-export-%s.xlsx", time.Now().Format("20060102")))
}

func (a *AppContext) adminGenerateWeeklySummary(c *gin.Context) {
	_, err := a.generateAndCacheWeeklySummary(c.Request.Context())
	if err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/reports?msg=周报已重新生成")
}

func (a *AppContext) adminDownloadPDF(c *gin.Context) {
	summary, ok := a.GetWeeklySummary()
	if !ok {
		generated, err := a.generateAndCacheWeeklySummary(c.Request.Context())
		if err != nil {
			c.Redirect(http.StatusFound, "/reports?error="+err.Error())
			return
		}
		summary = generated
	}

	pdfPath, err := utils.GenerateWeeklyReportPDF(utils.WeeklyReportPDFData{
		PeriodStart:     summary.PeriodStart,
		PeriodEnd:       summary.PeriodEnd,
		GeneratedAt:     summary.GeneratedAt,
		DutyCount:       summary.DutyCount,
		TicketCount:     summary.TicketCount,
		WorkTicketCount: summary.WorkTicketCount,
		Summary:         summary.Summary,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+err.Error())
		return
	}
	defer os.Remove(pdfPath)

	c.FileAttachment(pdfPath, fmt.Sprintf("weekly-report-%s.pdf", time.Now().Format("20060102")))
}

func (a *AppContext) generateAndCacheWeeklySummary(ctx context.Context) (utils.WeeklySummaryResult, error) {
	result, err := utils.GenerateWeeklySummary(ctx, a.DB, a.ConfigCenter, time.Now())
	if err != nil {
		return utils.WeeklySummaryResult{}, err
	}
	a.SetWeeklySummary(result)
	return result, nil
}

func (a *AppContext) exportIDCSheet(file *excelize.File, sheetName string, currentUser *models.User) error {
	headers := []string{"日期", "运维值班", "机房值班", "事项内容", "创建时间"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		file.SetCellValue(sheetName, cell, header)
	}

	var rows []models.IdcDutyRecord
	query := applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Order("date desc")
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	for i, row := range rows {
		r := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(r), row.Date.Format("2006-01-02"))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(r), row.DutyOps)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(r), row.DutyIdc)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(r), row.Tasks)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(r), row.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func (a *AppContext) exportWorkTicketSheet(file *excelize.File, sheetName string, currentUser *models.User) error {
	headers := []string{"日期", "值班人员", "用户名", "所属单位", "工单类型ID", "操作信息", "处理状态", "创建时间"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		file.SetCellValue(sheetName, cell, header)
	}

	var rows []models.WorkTicket
	query := applyUserScope(a.DB.Model(&models.WorkTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Order("date desc")
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	for i, row := range rows {
		r := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(r), row.Date.Format("2006-01-02"))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(r), row.DutyPerson)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(r), row.UserName)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(r), row.TicketOrganization)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(r), row.WorkTicketTypeID)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(r), row.OperationInfo)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(r), row.ProcessingStatus)
		file.SetCellValue(sheetName, "H"+strconv.Itoa(r), row.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func (a *AppContext) exportFaultSheet(file *excelize.File, sheetName string, currentUser *models.User) error {
	headers := []string{"日期", "值班人员", "状态", "用户名", "故障类型ID", "故障现象", "处理过程", "处理状态", "受理时间", "完成时间", "创建时间"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		file.SetCellValue(sheetName, cell, header)
	}

	var rows []models.FaultRecord
	query := applyUserScope(a.DB.Model(&models.FaultRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Order("date desc")
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	for i, row := range rows {
		r := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(r), row.Date.Format("2006-01-02"))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(r), row.DutyPerson)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(r), row.Status)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(r), row.UserName)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(r), row.FaultTypeID)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(r), row.FaultSymptom)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(r), row.ProcessingProcess)
		file.SetCellValue(sheetName, "H"+strconv.Itoa(r), row.ProcessingStatus)
		file.SetCellValue(sheetName, "I"+strconv.Itoa(r), row.ReceivedTime.Format("2006-01-02 15:04:05"))
		if row.CompletedTime != nil {
			file.SetCellValue(sheetName, "J"+strconv.Itoa(r), row.CompletedTime.Format("2006-01-02 15:04:05"))
		}
		file.SetCellValue(sheetName, "K"+strconv.Itoa(r), row.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func canUseGlobalWeeklySummary(isAdmin bool) bool {
	return isAdmin
}

func statisticsUserCountLabel(isAdmin bool) string {
	if isAdmin {
		return "系统用户"
	}
	return "我的账号"
}

func applyUserScope(query *gorm.DB, isAdmin bool, userID uint, userColumn string) *gorm.DB {
	if isAdmin {
		return query
	}
	return query.Where(userColumn+" = ?", userID)
}
