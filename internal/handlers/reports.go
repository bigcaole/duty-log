package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"
	"duty-log-system/internal/scheduler"
	"duty-log-system/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type reportPeriodOption struct {
	Key   string
	Label string
}

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

	period := normalizeReportPeriod(c.Query("period"))
	summary := utils.WeeklySummaryResult{}
	hasData := false

	if canUseGlobalWeeklySummary(currentUser.IsAdmin) {
		summary, hasData = a.GetSummary(period)
		if !hasData {
			generated, genErr := a.generateAndCacheSummary(c.Request.Context(), period)
			if genErr == nil {
				summary = generated
				hasData = true
			}
		}
	}

	c.HTML(http.StatusOK, "reports/report.html", gin.H{
		"Title":         "报表中心",
		"HasData":       hasData,
		"Summary":       summary,
		"IsAdmin":       currentUser.IsAdmin,
		"CurrentPeriod": period,
		"PeriodOptions": availableReportPeriods(),
		"Msg":           strings.TrimSpace(c.Query("msg")),
		"Error":         strings.TrimSpace(c.Query("error")),
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
	var idcOpsCount int64
	var workTicketCount int64
	var faultCount int64
	if currentUser.IsAdmin {
		_ = a.DB.Model(&models.User{}).Count(&userCount).Error
	} else {
		userCount = 1
	}
	_ = applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&dutyCount).Error
	_ = applyUserScope(a.DB.Model(&models.IDCOpsTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&idcOpsCount).Error
	_ = applyUserScope(a.DB.Model(&models.WorkTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&workTicketCount).Error
	_ = applyUserScope(a.DB.Model(&models.FaultRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").Count(&faultCount).Error

	now := time.Now()
	start7 := now.AddDate(0, 0, -7).Format("2006-01-02")
	end := now.Format("2006-01-02")

	var recentDuty int64
	var recentIDCOps int64
	var recentWork int64
	var recentFault int64
	_ = applyUserScope(a.DB.Model(&models.IdcDutyRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentDuty).Error
	_ = applyUserScope(a.DB.Model(&models.IDCOpsTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentIDCOps).Error
	_ = applyUserScope(a.DB.Model(&models.WorkTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentWork).Error
	_ = applyUserScope(a.DB.Model(&models.FaultRecord{}), currentUser.IsAdmin, currentUser.ID, "user_id").
		Where("date >= ? AND date <= ?", start7, end).
		Count(&recentFault).Error

	charts := []chartItem{
		{Name: "IDC值班记录", Count: dutyCount},
		{Name: "IDC运维工单", Count: idcOpsCount},
		{Name: "网络运维工单", Count: workTicketCount},
		{Name: "网络故障记录", Count: faultCount},
	}

	c.HTML(http.StatusOK, "reports/statistics.html", gin.H{
		"Title":           "统计页面",
		"UserCount":       userCount,
		"UserCountLabel":  statisticsUserCountLabel(currentUser.IsAdmin),
		"DutyCount":       dutyCount,
		"IDCOpsCount":     idcOpsCount,
		"WorkTicketCount": workTicketCount,
		"FaultCount":      faultCount,
		"RecentDuty":      recentDuty,
		"RecentIDCOps":    recentIDCOps,
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
	workbook.NewSheet("IDC Ops Tickets")
	workbook.NewSheet("Network Work Tickets")
	workbook.NewSheet("Network Fault Records")

	if err := a.exportIDCSheet(workbook, "IDC Duty", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	if err := a.exportIDCOpsTicketSheet(workbook, "IDC Ops Tickets", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	if err := a.exportWorkTicketSheet(workbook, "Network Work Tickets", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	if err := a.exportFaultSheet(workbook, "Network Fault Records", currentUser); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}

	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("duty-log-export-%s.xlsx", time.Now().Format("20060102-150405")))
	if err := workbook.SaveAs(tempPath); err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	defer os.Remove(tempPath)

	c.FileAttachment(tempPath, fmt.Sprintf("duty-log-export-%s.xlsx", time.Now().Format("20060102")))
}

func (a *AppContext) adminGenerateWeeklySummary(c *gin.Context) {
	period := normalizeReportPeriod(c.PostForm("period"))
	result, err := a.generateAndCacheSummary(c.Request.Context(), period)
	if err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/reports?period="+url.QueryEscape(period)+"&msg="+url.QueryEscape(result.ReportTypeLabel+"已重新生成"))
}

func (a *AppContext) adminDownloadPDF(c *gin.Context) {
	period := normalizeReportPeriod(c.Query("period"))
	summary, ok := a.GetSummary(period)
	if !ok {
		generated, err := a.generateAndCacheSummary(c.Request.Context(), period)
		if err != nil {
			c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
			return
		}
		summary = generated
	}

	pdfPath, err := utils.GenerateWeeklyReportPDF(utils.WeeklyReportPDFData{
		ReportTypeLabel:   summary.ReportTypeLabel,
		PeriodStart:       summary.PeriodStart,
		PeriodEnd:         summary.PeriodEnd,
		GeneratedAt:       summary.GeneratedAt,
		DutyCount:         summary.DutyCount,
		IDCOpsTicketCount: summary.IDCOpsTicketCount,
		WorkTicketCount:   summary.WorkTicketCount,
		NetworkFaultCount: summary.NetworkFaultCount,
		Summary:           summary.Summary,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}
	defer os.Remove(pdfPath)

	fileName := fmt.Sprintf("%s-%s.pdf", period, time.Now().Format("20060102"))
	c.FileAttachment(pdfPath, fileName)
}

func (a *AppContext) adminTestWeeklyDelivery(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	result, err := scheduler.RunWeeklyReportJob(
		ctx,
		a.DB,
		a.ConfigCenter,
		scheduler.WeeklyReportRunOptions{ForceNotify: true},
		a.SetWeeklySummary,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/reports?error="+url.QueryEscape(err.Error()))
		return
	}

	msg := fmt.Sprintf(
		"自动周报推送测试完成：邮件(%t/%t)，飞书(%t/%t)",
		result.EmailAttempted,
		result.EmailSent,
		result.FeishuAttempted,
		result.FeishuSent,
	)
	c.Redirect(http.StatusFound, "/reports?msg="+url.QueryEscape(msg))
}

func (a *AppContext) generateAndCacheSummary(ctx context.Context, period string) (utils.WeeklySummaryResult, error) {
	result, err := utils.GeneratePeriodicSummary(ctx, a.DB, a.ConfigCenter, time.Now(), period)
	if err != nil {
		return utils.WeeklySummaryResult{}, err
	}
	a.SetSummary(period, result)
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

func (a *AppContext) exportIDCOpsTicketSheet(file *excelize.File, sheetName string, currentUser *models.User) error {
	headers := []string{"日期", "来访单位", "来访人数", "来访事由", "客服人员", "备注", "附件数", "创建时间"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		file.SetCellValue(sheetName, cell, header)
	}

	var rows []models.IDCOpsTicket
	query := applyUserScope(a.DB.Model(&models.IDCOpsTicket{}), currentUser.IsAdmin, currentUser.ID, "user_id").Order("date desc")
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	for i, row := range rows {
		r := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(r), row.Date.Format("2006-01-02"))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(r), row.VisitorOrganization)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(r), row.VisitorCount)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(r), row.VisitorReason)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(r), row.CustomerServicePerson)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(r), row.Remarks)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(r), len(row.AttachmentsJSON))
		file.SetCellValue(sheetName, "H"+strconv.Itoa(r), row.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func (a *AppContext) exportWorkTicketSheet(file *excelize.File, sheetName string, currentUser *models.User) error {
	headers := []string{"日期", "值班人员", "用户名", "所属单位", "工单类型ID", "操作信息", "处理状态", "附件数", "创建时间"}
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
		file.SetCellValue(sheetName, "H"+strconv.Itoa(r), len(row.AttachmentsJSON))
		file.SetCellValue(sheetName, "I"+strconv.Itoa(r), row.CreatedAt.Format("2006-01-02 15:04:05"))
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

func normalizeReportPeriod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
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

func availableReportPeriods() []reportPeriodOption {
	return []reportPeriodOption{
		{Key: "week", Label: "周报"},
		{Key: "month", Label: "月报"},
		{Key: "halfyear", Label: "半年报"},
		{Key: "year", Label: "年报"},
	}
}

func applyUserScope(query *gorm.DB, isAdmin bool, userID uint, userColumn string) *gorm.DB {
	if isAdmin {
		return query
	}
	return query.Where(userColumn+" = ?", userID)
}
