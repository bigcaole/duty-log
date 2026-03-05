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
)

type workTicketListItem struct {
	ID                 uint
	Date               string
	DutyPerson         string
	UserName           string
	Organization       string
	WorkTicketTypeName string
	ProcessingStatus   string
	UpdatedAt          string
}

type workTicketFormView struct {
	ID                    uint
	Date                  string
	DutyPerson            string
	UserName              string
	TicketOrganization    string
	WorkTicketTypeID      string
	OperationInfo         string
	CustomerServicePerson string
	ProcessingStatus      string
	Remarks               string
}

func registerWorkTicketRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/work-tickets", app.workTicketList)
	group.GET("/work-tickets/create", app.workTicketCreatePage)
	group.POST("/work-tickets/create", app.workTicketCreate)
	group.GET("/work-tickets/:id/edit", app.workTicketEditPage)
	group.POST("/work-tickets/:id/edit", app.workTicketUpdate)
	group.POST("/work-tickets/:id/delete", app.workTicketDelete)
}

func (a *AppContext) workTicketList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var records []models.WorkTicket
	query := a.DB.Order("date desc, id desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "网络运维工单",
			"Path":    "/work-tickets",
			"Message": "读取工单失败：" + err.Error(),
		})
		return
	}

	var ticketTypes []models.WorkTicketType
	_ = a.DB.Order("name asc").Find(&ticketTypes).Error
	typeNameByID := make(map[uint]string, len(ticketTypes))
	for _, item := range ticketTypes {
		typeNameByID[item.ID] = item.Name
	}

	items := make([]workTicketListItem, 0, len(records))
	for _, record := range records {
		typeName := "-"
		if name, ok := typeNameByID[record.WorkTicketTypeID]; ok {
			typeName = name
		}
		items = append(items, workTicketListItem{
			ID:                 record.ID,
			Date:               record.Date.Format(dateLayout),
			DutyPerson:         record.DutyPerson,
			UserName:           record.UserName,
			Organization:       record.TicketOrganization,
			WorkTicketTypeName: typeName,
			ProcessingStatus:   record.ProcessingStatus,
			UpdatedAt:          record.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "work_ticket/list.html", gin.H{
		"Title":   "网络运维工单",
		"Items":   items,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) workTicketCreatePage(c *gin.Context) {
	a.renderWorkTicketForm(c, http.StatusOK, "新建网络运维工单", "/work-tickets/create", workTicketFormView{
		Date:             todayDateString(),
		ProcessingStatus: "pending",
	}, "")
}

func (a *AppContext) workTicketCreate(c *gin.Context) {
	record, formView, err := a.bindWorkTicketForm(c)
	if err != nil {
		a.renderWorkTicketForm(c, http.StatusBadRequest, "新建网络运维工单", "/work-tickets/create", formView, err.Error())
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok || userID == 0 {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	record.UserID = userID

	if err := a.DB.Create(&record).Error; err != nil {
		a.renderWorkTicketForm(c, http.StatusBadRequest, "新建网络运维工单", "/work-tickets/create", formView, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/work-tickets?msg=创建成功")
}

func (a *AppContext) workTicketEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/work-tickets?error=无效工单ID")
		return
	}

	var record models.WorkTicket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/work-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/work-tickets?error=无权编辑他人工单")
		return
	}

	formView := workTicketFormView{
		ID:                    record.ID,
		Date:                  record.Date.Format(dateLayout),
		DutyPerson:            record.DutyPerson,
		UserName:              record.UserName,
		TicketOrganization:    record.TicketOrganization,
		WorkTicketTypeID:      strconv.FormatUint(uint64(record.WorkTicketTypeID), 10),
		OperationInfo:         record.OperationInfo,
		CustomerServicePerson: record.CustomerServicePerson,
		ProcessingStatus:      record.ProcessingStatus,
		Remarks:               record.Remarks,
	}
	a.renderWorkTicketForm(c, http.StatusOK, "编辑网络运维工单", "/work-tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) workTicketUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/work-tickets?error=无效工单ID")
		return
	}

	var existing models.WorkTicket
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/work-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/work-tickets?error=无权编辑他人工单")
		return
	}

	record, formView, bindErr := a.bindWorkTicketForm(c)
	formView.ID = existing.ID
	if bindErr != nil {
		a.renderWorkTicketForm(c, http.StatusBadRequest, "编辑网络运维工单", "/work-tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	existing.Date = record.Date
	existing.DutyPerson = record.DutyPerson
	existing.UserName = record.UserName
	existing.TicketOrganization = record.TicketOrganization
	existing.WorkTicketTypeID = record.WorkTicketTypeID
	existing.OperationInfo = record.OperationInfo
	existing.CustomerServicePerson = record.CustomerServicePerson
	existing.ProcessingStatus = record.ProcessingStatus
	existing.Remarks = record.Remarks
	existing.UpdatedAt = time.Now()

	if err := a.DB.Save(&existing).Error; err != nil {
		a.renderWorkTicketForm(c, http.StatusBadRequest, "编辑网络运维工单", "/work-tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/work-tickets?msg=更新成功")
}

func (a *AppContext) workTicketDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/work-tickets?error=无效工单ID")
		return
	}
	var record models.WorkTicket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/work-tickets?error=工单不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/work-tickets?error=无权删除他人工单")
		return
	}

	if err := a.DB.Delete(&models.WorkTicket{}, record.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/work-tickets?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/work-tickets?msg=删除成功")
}

func (a *AppContext) bindWorkTicketForm(c *gin.Context) (models.WorkTicket, workTicketFormView, error) {
	formView := workTicketFormView{
		Date:                  strings.TrimSpace(c.PostForm("date")),
		DutyPerson:            strings.TrimSpace(c.PostForm("duty_person")),
		UserName:              strings.TrimSpace(c.PostForm("user_name")),
		TicketOrganization:    strings.TrimSpace(c.PostForm("ticket_organization")),
		WorkTicketTypeID:      strings.TrimSpace(c.PostForm("work_ticket_type_id")),
		OperationInfo:         strings.TrimSpace(c.PostForm("operation_info")),
		CustomerServicePerson: strings.TrimSpace(c.PostForm("customer_service_person")),
		ProcessingStatus:      strings.TrimSpace(c.PostForm("processing_status")),
		Remarks:               strings.TrimSpace(c.PostForm("remarks")),
	}

	date, err := parseRequiredDate(formView.Date)
	if err != nil {
		return models.WorkTicket{}, formView, err
	}
	if formView.DutyPerson == "" {
		return models.WorkTicket{}, formView, fmt.Errorf("值班人员不能为空")
	}
	if formView.UserName == "" {
		return models.WorkTicket{}, formView, fmt.Errorf("用户名称不能为空")
	}
	typeID, err := parseRequiredUint(formView.WorkTicketTypeID, "工单类型")
	if err != nil {
		return models.WorkTicket{}, formView, err
	}
	if formView.OperationInfo == "" {
		return models.WorkTicket{}, formView, fmt.Errorf("操作信息不能为空")
	}
	if formView.ProcessingStatus == "" {
		formView.ProcessingStatus = "pending"
	}

	record := models.WorkTicket{
		Date:                  date,
		DutyPerson:            formView.DutyPerson,
		UserName:              formView.UserName,
		TicketOrganization:    formView.TicketOrganization,
		WorkTicketTypeID:      typeID,
		OperationInfo:         formView.OperationInfo,
		CustomerServicePerson: formView.CustomerServicePerson,
		ProcessingStatus:      formView.ProcessingStatus,
		Remarks:               formView.Remarks,
	}
	return record, formView, nil
}

func (a *AppContext) renderWorkTicketForm(c *gin.Context, statusCode int, title, action string, formView workTicketFormView, errorMessage string) {
	var ticketTypes []models.WorkTicketType
	_ = a.DB.Order("name asc").Find(&ticketTypes).Error

	c.HTML(statusCode, "work_ticket/form.html", gin.H{
		"Title":       title,
		"Action":      action,
		"Form":        formView,
		"TicketTypes": ticketTypes,
		"Error":       errorMessage,
		"Statuses":    []string{"pending", "processing", "completed"},
	})
}
