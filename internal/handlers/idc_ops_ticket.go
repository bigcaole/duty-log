package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type idcOpsTicketListItem struct {
	ID                  uint
	Date                string
	VisitorOrganization string
	VisitorCount        int
	VisitorReason       string
	CustomerService     string
	Remarks             string
	AttachmentCount     int
	UpdatedAt           string
	CanEdit             bool
}

type idcOpsTicketFormView struct {
	ID                  uint
	Date                string
	VisitorOrganization string
	VisitorCount        string
	VisitorReason       string
	CustomerService     string
	Remarks             string
	Attachments         []attachmentViewItem
	ReminderEnabled     bool
	ReminderDate        string
	ReminderTime        string
	ReminderTitle       string
	ReminderContent     string
	ReminderDaysBefore  string
}

func registerIDCOpsTicketRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/idc-ops-tickets", app.idcOpsTicketList)
	group.GET("/idc-ops-tickets/create", app.idcOpsTicketCreatePage)
	group.POST("/idc-ops-tickets/create", app.idcOpsTicketCreate)
	group.GET("/idc-ops-tickets/:id/edit", app.idcOpsTicketEditPage)
	group.POST("/idc-ops-tickets/:id/edit", app.idcOpsTicketUpdate)
	group.POST("/idc-ops-tickets/:id/delete", app.idcOpsTicketDelete)
}

func (a *AppContext) idcOpsTicketList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var rows []models.IDCOpsTicket
	query := a.DB.Order("date desc, id desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if parsed, err := parseOptionalDate(dateFrom); err == nil && parsed != nil {
		query = query.Where("date >= ?", parsed.Format(dateLayout))
	}
	if parsed, err := parseOptionalDate(dateTo); err == nil && parsed != nil {
		query = query.Where("date <= ?", parsed.Format(dateLayout))
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"visitor_organization ILIKE ? OR visitor_reason ILIKE ? OR customer_service_person ILIKE ? OR remarks ILIKE ?",
			like, like, like, like,
		)
	}
	if err := query.Find(&rows).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "IDC运维工单",
			"Path":    "/idc-ops-tickets",
			"Message": "读取 IDC 运维工单失败：" + err.Error(),
		})
		return
	}

	items := make([]idcOpsTicketListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, idcOpsTicketListItem{
			ID:                  row.ID,
			Date:                row.Date.Format(dateLayout),
			VisitorOrganization: row.VisitorOrganization,
			VisitorCount:        row.VisitorCount,
			VisitorReason:       trimDashboardText(row.VisitorReason, 80),
			CustomerService:     row.CustomerServicePerson,
			Remarks:             trimDashboardText(row.Remarks, 80),
			AttachmentCount:     len(row.AttachmentsJSON),
			UpdatedAt:           row.UpdatedAt.Format("2006-01-02 15:04"),
			CanEdit:             currentUser.IsAdmin || row.UserID == currentUser.ID,
		})
	}

	c.HTML(http.StatusOK, "idc_ops_ticket/list.html", gin.H{
		"Title": "IDC运维工单",
		"Items": items,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
		"Filter": gin.H{
			"DateFrom": dateFrom,
			"DateTo":   dateTo,
			"Keyword":  keyword,
		},
	})
}

func (a *AppContext) idcOpsTicketCreatePage(c *gin.Context) {
	form := idcOpsTicketFormView{
		Date:         todayDateString(),
		VisitorCount: "1",
	}
	a.renderIDCOpsTicketForm(c, http.StatusOK, "新建 IDC运维工单", "/idc-ops-tickets/create", form, "")
}

func (a *AppContext) idcOpsTicketCreate(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok || userID == 0 {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	record, form, err := bindIDCOpsTicketForm(c)
	if err != nil {
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "新建 IDC运维工单", "/idc-ops-tickets/create", form, err.Error())
		return
	}

	attachments, attachErr := saveUploadedAttachments(c, "attachments", "idc_ops_ticket")
	if attachErr != nil {
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "新建 IDC运维工单", "/idc-ops-tickets/create", form, "附件上传失败："+attachErr.Error())
		return
	}

	record.UserID = userID
	record.AttachmentsJSON = attachments
	reminderReq := readReminderRequest(c)
	form.ReminderEnabled = reminderReq.Enabled
	form.ReminderDate = reminderReq.Date
	form.ReminderTime = reminderReq.Time
	form.ReminderTitle = reminderReq.Title
	form.ReminderContent = reminderReq.Content
	form.ReminderDaysBefore = reminderReq.DaysBefore

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		reminder, err := buildReminderFromRequest(
			reminderReq,
			record.Date,
			userID,
			fmt.Sprintf("IDC运维工单提醒：%s", record.VisitorOrganization),
			fmt.Sprintf("来访人数: %d\n来访事由: %s", record.VisitorCount, record.VisitorReason),
		)
		if err != nil {
			return err
		}
		if reminder != nil {
			if err := tx.Create(reminder).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "新建 IDC运维工单", "/idc-ops-tickets/create", form, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/idc-ops-tickets?msg=创建成功")
}

func (a *AppContext) idcOpsTicketEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无效工单ID")
		return
	}

	var record models.IDCOpsTicket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无权编辑他人工单")
		return
	}

	form := idcOpsTicketFormView{
		ID:                  record.ID,
		Date:                record.Date.Format(dateLayout),
		VisitorOrganization: record.VisitorOrganization,
		VisitorCount:        strconv.Itoa(record.VisitorCount),
		VisitorReason:       record.VisitorReason,
		CustomerService:     record.CustomerServicePerson,
		Remarks:             record.Remarks,
		Attachments:         parseAttachmentViewItems(record.AttachmentsJSON),
	}
	a.renderIDCOpsTicketForm(c, http.StatusOK, "编辑 IDC运维工单", "/idc-ops-tickets/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) idcOpsTicketUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无效工单ID")
		return
	}

	var existing models.IDCOpsTicket
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无权编辑他人工单")
		return
	}

	record, form, bindErr := bindIDCOpsTicketForm(c)
	form.ID = existing.ID
	if bindErr != nil {
		form.Attachments = parseAttachmentViewItems(existing.AttachmentsJSON)
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "编辑 IDC运维工单", "/idc-ops-tickets/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}

	newAttachments, attachErr := saveUploadedAttachments(c, "attachments", "idc_ops_ticket")
	if attachErr != nil {
		form.Attachments = parseAttachmentViewItems(existing.AttachmentsJSON)
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "编辑 IDC运维工单", "/idc-ops-tickets/"+strconv.FormatUint(id, 10)+"/edit", form, "附件上传失败："+attachErr.Error())
		return
	}

	existing.Date = record.Date
	existing.VisitorOrganization = record.VisitorOrganization
	existing.VisitorCount = record.VisitorCount
	existing.VisitorReason = record.VisitorReason
	existing.CustomerServicePerson = record.CustomerServicePerson
	existing.Remarks = record.Remarks
	existing.AttachmentsJSON = mergeAttachments(existing.AttachmentsJSON, newAttachments)
	existing.UpdatedAt = time.Now()

	reminderReq := readReminderRequest(c)
	form.ReminderEnabled = reminderReq.Enabled
	form.ReminderDate = reminderReq.Date
	form.ReminderTime = reminderReq.Time
	form.ReminderTitle = reminderReq.Title
	form.ReminderContent = reminderReq.Content
	form.ReminderDaysBefore = reminderReq.DaysBefore

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		reminder, err := buildReminderFromRequest(
			reminderReq,
			existing.Date,
			currentUser.ID,
			fmt.Sprintf("IDC运维工单提醒：%s", existing.VisitorOrganization),
			fmt.Sprintf("来访人数: %d\n来访事由: %s", existing.VisitorCount, existing.VisitorReason),
		)
		if err != nil {
			return err
		}
		if reminder != nil {
			if err := tx.Create(reminder).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		form.Attachments = parseAttachmentViewItems(existing.AttachmentsJSON)
		a.renderIDCOpsTicketForm(c, http.StatusBadRequest, "编辑 IDC运维工单", "/idc-ops-tickets/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/idc-ops-tickets?msg=更新成功")
}

func (a *AppContext) idcOpsTicketDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无效工单ID")
		return
	}

	var existing models.IDCOpsTicket
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error=无权删除他人工单")
		return
	}

	if err := a.DB.Delete(&models.IDCOpsTicket{}, existing.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-ops-tickets?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/idc-ops-tickets?msg=删除成功")
}

func bindIDCOpsTicketForm(c *gin.Context) (models.IDCOpsTicket, idcOpsTicketFormView, error) {
	reminderReq := readReminderRequest(c)
	form := idcOpsTicketFormView{
		Date:                strings.TrimSpace(c.PostForm("date")),
		VisitorOrganization: strings.TrimSpace(c.PostForm("visitor_organization")),
		VisitorCount:        strings.TrimSpace(c.PostForm("visitor_count")),
		VisitorReason:       strings.TrimSpace(c.PostForm("visitor_reason")),
		CustomerService:     strings.TrimSpace(c.PostForm("customer_service_person")),
		Remarks:             strings.TrimSpace(c.PostForm("remarks")),
		ReminderEnabled:     reminderReq.Enabled,
		ReminderDate:        reminderReq.Date,
		ReminderTime:        reminderReq.Time,
		ReminderTitle:       reminderReq.Title,
		ReminderContent:     reminderReq.Content,
		ReminderDaysBefore:  reminderReq.DaysBefore,
	}

	date, err := parseRequiredDate(form.Date)
	if err != nil {
		return models.IDCOpsTicket{}, form, err
	}
	if form.VisitorOrganization == "" {
		return models.IDCOpsTicket{}, form, fmt.Errorf("来访人员单位不能为空")
	}
	visitorCount, err := strconv.Atoi(form.VisitorCount)
	if err != nil || visitorCount <= 0 {
		return models.IDCOpsTicket{}, form, fmt.Errorf("来访人员数量必须为正整数")
	}
	if form.VisitorReason == "" {
		return models.IDCOpsTicket{}, form, fmt.Errorf("来访人员事由不能为空")
	}

	return models.IDCOpsTicket{
		Date:                  date,
		VisitorOrganization:   form.VisitorOrganization,
		VisitorCount:          visitorCount,
		VisitorReason:         form.VisitorReason,
		CustomerServicePerson: form.CustomerService,
		Remarks:               form.Remarks,
	}, form, nil
}

func (a *AppContext) renderIDCOpsTicketForm(c *gin.Context, statusCode int, title, action string, form idcOpsTicketFormView, errMsg string) {
	if strings.TrimSpace(form.VisitorCount) == "" {
		form.VisitorCount = "1"
	}
	if strings.TrimSpace(form.ReminderDaysBefore) == "" {
		form.ReminderDaysBefore = "2"
	}
	if strings.TrimSpace(form.ReminderTime) == "" {
		form.ReminderTime = "09:00"
	}
	c.HTML(statusCode, "idc_ops_ticket/form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errMsg,
	})
}
