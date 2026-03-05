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

type ticketListItem struct {
	ID           uint
	Title        string
	CategoryName string
	Status       string
	Priority     string
	Creator      string
	CreatedAt    string
	UpdatedAt    string
}

type ticketFormView struct {
	ID               uint
	Title            string
	Content          string
	TicketCategoryID string
	Status           string
	Priority         string
}

type ticketHistoryItem struct {
	Status      string
	ChangedBy   string
	ChangedAt   string
	ChangedByID *uint
}

func registerTicketRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/tickets", app.ticketList)
	group.GET("/tickets/create", app.ticketCreatePage)
	group.POST("/tickets/create", app.ticketCreate)
	group.GET("/tickets/:id", app.ticketDetailPage)
	group.GET("/tickets/:id/edit", app.ticketEditPage)
	group.POST("/tickets/:id/edit", app.ticketUpdate)
	group.POST("/tickets/:id/delete", app.ticketDelete)
}

func (a *AppContext) ticketList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var tickets []models.Ticket
	query := a.DB.Order("created_at desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.Find(&tickets).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "普通工单",
			"Path":    "/tickets",
			"Message": "读取普通工单失败：" + err.Error(),
		})
		return
	}

	var categories []models.TicketCategory
	_ = a.DB.Order("name asc").Find(&categories).Error
	categoryNameByID := make(map[uint]string, len(categories))
	for _, item := range categories {
		categoryNameByID[item.ID] = item.Name
	}

	userIDs := make([]uint, 0, len(tickets))
	userIDSet := make(map[uint]struct{})
	for _, ticket := range tickets {
		if _, ok := userIDSet[ticket.UserID]; !ok {
			userIDs = append(userIDs, ticket.UserID)
			userIDSet[ticket.UserID] = struct{}{}
		}
	}
	var users []models.User
	if len(userIDs) > 0 {
		_ = a.DB.Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error
	}
	usernameByID := make(map[uint]string, len(users))
	for _, user := range users {
		usernameByID[user.ID] = user.Username
	}

	items := make([]ticketListItem, 0, len(tickets))
	for _, ticket := range tickets {
		categoryName := "-"
		if ticket.TicketCategoryID != nil {
			if name, ok := categoryNameByID[*ticket.TicketCategoryID]; ok {
				categoryName = name
			}
		}
		creator := "-"
		if name, ok := usernameByID[ticket.UserID]; ok {
			creator = name
		}
		items = append(items, ticketListItem{
			ID:           ticket.ID,
			Title:        ticket.Title,
			CategoryName: categoryName,
			Status:       ticket.Status,
			Priority:     ticket.Priority,
			Creator:      creator,
			CreatedAt:    ticket.CreatedAt.Format("2006-01-02 15:04"),
			UpdatedAt:    ticket.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "ticket/list.html", gin.H{
		"Title":   "普通工单",
		"Items":   items,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) ticketCreatePage(c *gin.Context) {
	a.renderTicketForm(c, http.StatusOK, "新建普通工单", "/tickets/create", ticketFormView{
		Status:   "open",
		Priority: "medium",
	}, "")
}

func (a *AppContext) ticketCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	record, formView, bindErr := bindTicketForm(c)
	if bindErr != nil {
		a.renderTicketForm(c, http.StatusBadRequest, "新建普通工单", "/tickets/create", formView, bindErr.Error())
		return
	}

	record.UserID = currentUser.ID
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderTicketForm(c, http.StatusBadRequest, "新建普通工单", "/tickets/create", formView, "创建失败："+err.Error())
		return
	}

	_ = a.DB.Create(&models.TicketStatusHistory{
		TicketID:  record.ID,
		Status:    record.Status,
		ChangedBy: &currentUser.ID,
	}).Error

	c.Redirect(http.StatusFound, "/tickets?msg=创建成功")
}

func (a *AppContext) ticketDetailPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/tickets?error=无效工单ID")
		return
	}

	var record models.Ticket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/tickets?error=工单不存在")
		return
	}
	if !currentUser.IsAdmin && record.UserID != currentUser.ID {
		c.Redirect(http.StatusFound, "/tickets?error=无权查看他人工单")
		return
	}

	var categoryName string
	if record.TicketCategoryID != nil {
		var cat models.TicketCategory
		if err := a.DB.First(&cat, *record.TicketCategoryID).Error; err == nil {
			categoryName = cat.Name
		}
	}
	if strings.TrimSpace(categoryName) == "" {
		categoryName = "-"
	}

	var histories []models.TicketStatusHistory
	_ = a.DB.Where("ticket_id = ?", record.ID).Order("created_at asc").Find(&histories).Error

	userIDSet := make(map[uint]struct{})
	userIDs := make([]uint, 0, len(histories)+1)
	if _, ok := userIDSet[record.UserID]; !ok {
		userIDs = append(userIDs, record.UserID)
		userIDSet[record.UserID] = struct{}{}
	}
	for _, h := range histories {
		if h.ChangedBy == nil {
			continue
		}
		if _, ok := userIDSet[*h.ChangedBy]; !ok {
			userIDs = append(userIDs, *h.ChangedBy)
			userIDSet[*h.ChangedBy] = struct{}{}
		}
	}

	var users []models.User
	if len(userIDs) > 0 {
		_ = a.DB.Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error
	}
	usernameByID := make(map[uint]string, len(users))
	for _, user := range users {
		usernameByID[user.ID] = user.Username
	}

	creatorName := "-"
	if name, ok := usernameByID[record.UserID]; ok {
		creatorName = name
	}

	historyItems := make([]ticketHistoryItem, 0, len(histories))
	for _, h := range histories {
		changedBy := "system"
		if h.ChangedBy != nil {
			if name, ok := usernameByID[*h.ChangedBy]; ok {
				changedBy = name
			} else {
				changedBy = fmt.Sprintf("user#%d", *h.ChangedBy)
			}
		}
		historyItems = append(historyItems, ticketHistoryItem{
			Status:      h.Status,
			ChangedBy:   changedBy,
			ChangedAt:   h.CreatedAt.Format("2006-01-02 15:04:05"),
			ChangedByID: h.ChangedBy,
		})
	}

	c.HTML(http.StatusOK, "ticket/detail.html", gin.H{
		"Title":        "工单详情",
		"Ticket":       record,
		"CategoryName": categoryName,
		"CreatorName":  creatorName,
		"History":      historyItems,
		"IsAdmin":      currentUser.IsAdmin,
	})
}

func (a *AppContext) ticketEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/tickets?error=无效工单ID")
		return
	}

	var record models.Ticket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/tickets?error=工单不存在")
		return
	}
	if !currentUser.IsAdmin && record.UserID != currentUser.ID {
		c.Redirect(http.StatusFound, "/tickets?error=无权编辑他人工单")
		return
	}

	formView := ticketFormView{
		ID:       record.ID,
		Title:    record.Title,
		Content:  record.Content,
		Status:   record.Status,
		Priority: record.Priority,
	}
	if record.TicketCategoryID != nil {
		formView.TicketCategoryID = strconv.FormatUint(uint64(*record.TicketCategoryID), 10)
	}
	a.renderTicketForm(c, http.StatusOK, "编辑普通工单", "/tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) ticketUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/tickets?error=无效工单ID")
		return
	}

	var existing models.Ticket
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/tickets?error=工单不存在")
		return
	}
	if !currentUser.IsAdmin && existing.UserID != currentUser.ID {
		c.Redirect(http.StatusFound, "/tickets?error=无权编辑他人工单")
		return
	}

	record, formView, bindErr := bindTicketForm(c)
	formView.ID = existing.ID
	if bindErr != nil {
		a.renderTicketForm(c, http.StatusBadRequest, "编辑普通工单", "/tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	oldStatus := existing.Status
	existing.Title = record.Title
	existing.Content = record.Content
	existing.TicketCategoryID = record.TicketCategoryID
	existing.Status = record.Status
	existing.Priority = record.Priority
	existing.UpdatedAt = time.Now()

	if err := a.DB.Save(&existing).Error; err != nil {
		a.renderTicketForm(c, http.StatusBadRequest, "编辑普通工单", "/tickets/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}

	if strings.TrimSpace(strings.ToLower(oldStatus)) != strings.TrimSpace(strings.ToLower(existing.Status)) {
		changedBy := currentUser.ID
		_ = a.DB.Create(&models.TicketStatusHistory{
			TicketID:  existing.ID,
			Status:    existing.Status,
			ChangedBy: &changedBy,
		}).Error
	}

	c.Redirect(http.StatusFound, "/tickets?msg=更新成功")
}

func (a *AppContext) ticketDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/tickets?error=无效工单ID")
		return
	}

	var record models.Ticket
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/tickets?error=工单不存在")
		return
	}
	if !currentUser.IsAdmin && record.UserID != currentUser.ID {
		c.Redirect(http.StatusFound, "/tickets?error=无权删除他人工单")
		return
	}

	if err := a.DB.Delete(&models.Ticket{}, record.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/tickets?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/tickets?msg=删除成功")
}

func bindTicketForm(c *gin.Context) (models.Ticket, ticketFormView, error) {
	formView := ticketFormView{
		Title:            strings.TrimSpace(c.PostForm("title")),
		Content:          strings.TrimSpace(c.PostForm("content")),
		TicketCategoryID: strings.TrimSpace(c.PostForm("ticket_category_id")),
		Status:           strings.TrimSpace(strings.ToLower(c.PostForm("status"))),
		Priority:         strings.TrimSpace(strings.ToLower(c.PostForm("priority"))),
	}

	if formView.Title == "" {
		return models.Ticket{}, formView, fmt.Errorf("标题不能为空")
	}
	if formView.Content == "" {
		return models.Ticket{}, formView, fmt.Errorf("内容不能为空")
	}
	categoryID, err := parseOptionalUint(formView.TicketCategoryID)
	if err != nil {
		return models.Ticket{}, formView, err
	}

	if formView.Status == "" {
		formView.Status = "open"
	}
	validStatuses := map[string]struct{}{
		"open":        {},
		"in_progress": {},
		"resolved":    {},
		"closed":      {},
	}
	if _, ok := validStatuses[formView.Status]; !ok {
		return models.Ticket{}, formView, fmt.Errorf("工单状态不合法")
	}

	if formView.Priority == "" {
		formView.Priority = "medium"
	}
	validPriorities := map[string]struct{}{
		"low":    {},
		"medium": {},
		"high":   {},
		"urgent": {},
	}
	if _, ok := validPriorities[formView.Priority]; !ok {
		return models.Ticket{}, formView, fmt.Errorf("优先级不合法")
	}

	record := models.Ticket{
		Title:            formView.Title,
		Content:          formView.Content,
		TicketCategoryID: categoryID,
		Status:           formView.Status,
		Priority:         formView.Priority,
	}
	return record, formView, nil
}

func (a *AppContext) renderTicketForm(c *gin.Context, statusCode int, title, action string, formView ticketFormView, errorMessage string) {
	var categories []models.TicketCategory
	_ = a.DB.Order("name asc").Find(&categories).Error

	c.HTML(statusCode, "ticket/form.html", gin.H{
		"Title":      title,
		"Action":     action,
		"Form":       formView,
		"Categories": categories,
		"Statuses":   []string{"open", "in_progress", "resolved", "closed"},
		"Priorities": []string{"low", "medium", "high", "urgent"},
		"Error":      errorMessage,
	})
}
