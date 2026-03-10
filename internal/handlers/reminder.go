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

type reminderListItem struct {
	ID               uint
	Title            string
	Content          string
	StartDate        string
	EndDate          string
	RemindDaysBefore int
	Creator          string
	StatusLabel      string
	StatusClass      string
	IsCompleted      bool
	UpdatedAt        string
	CanEdit          bool
}

type reminderFormView struct {
	ID               uint
	Title            string
	Content          string
	StartDate        string
	EndDate          string
	RemindDaysBefore string
}

func registerReminderRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/reminders", app.reminderList)
	group.GET("/reminders/create", app.reminderCreatePage)
	group.POST("/reminders/create", app.reminderCreate)
	group.GET("/reminders/:id/edit", app.reminderEditPage)
	group.POST("/reminders/:id/edit", app.reminderUpdate)
	group.POST("/reminders/:id/delete", app.reminderDelete)
	group.POST("/reminders/:id/toggle-complete", app.reminderToggleComplete)
}

func (a *AppContext) reminderList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var records []models.Reminder
	query := a.DB.Order("is_completed asc, end_date asc, updated_at desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "提醒事项",
			"Path":    "/reminders",
			"Message": "读取提醒事项失败：" + err.Error(),
		})
		return
	}

	userIDs := make([]uint, 0, len(records))
	userIDSet := make(map[uint]struct{}, len(records))
	for _, record := range records {
		if _, ok := userIDSet[record.UserID]; ok {
			continue
		}
		userIDs = append(userIDs, record.UserID)
		userIDSet[record.UserID] = struct{}{}
	}

	var users []models.User
	if len(userIDs) > 0 {
		_ = a.DB.Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error
	}
	usernameByID := make(map[uint]string, len(users))
	for _, user := range users {
		usernameByID[user.ID] = user.Username
	}

	now := time.Now()
	items := make([]reminderListItem, 0, len(records))
	for _, record := range records {
		statusLabel, statusClass := reminderStatusBadge(record, now)
		creator := "-"
		if username, ok := usernameByID[record.UserID]; ok {
			creator = username
		}
		items = append(items, reminderListItem{
			ID:               record.ID,
			Title:            record.Title,
			Content:          record.Content,
			StartDate:        record.StartDate.Format(dateLayout),
			EndDate:          record.EndDate.Format(dateLayout),
			RemindDaysBefore: record.RemindDaysBefore,
			Creator:          creator,
			StatusLabel:      statusLabel,
			StatusClass:      statusClass,
			IsCompleted:      record.IsCompleted,
			UpdatedAt:        record.UpdatedAt.Format("2006-01-02 15:04"),
			CanEdit:          currentUser.IsAdmin || record.UserID == currentUser.ID,
		})
	}

	c.HTML(http.StatusOK, "reminder/list.html", gin.H{
		"Title": "提醒事项",
		"Items": items,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) reminderCreatePage(c *gin.Context) {
	form := reminderFormView{
		StartDate:        todayDateString(),
		EndDate:          todayDateString(),
		RemindDaysBefore: "2",
	}
	a.renderReminderForm(c, http.StatusOK, "新建提醒", "/reminders/create", form, "")
}

func (a *AppContext) reminderCreate(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok || userID == 0 {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	record, formView, err := bindReminderForm(c)
	if err != nil {
		a.renderReminderForm(c, http.StatusBadRequest, "新建提醒", "/reminders/create", formView, err.Error())
		return
	}
	record.UserID = userID
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderReminderForm(c, http.StatusBadRequest, "新建提醒", "/reminders/create", formView, "创建失败："+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/reminders?msg=提醒创建成功")
}

func (a *AppContext) reminderEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/reminders?error=无效提醒ID")
		return
	}

	var record models.Reminder
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error=提醒不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/reminders?error=无权编辑他人提醒")
		return
	}

	form := reminderFormView{
		ID:               record.ID,
		Title:            record.Title,
		Content:          record.Content,
		StartDate:        record.StartDate.Format(dateLayout),
		EndDate:          record.EndDate.Format(dateLayout),
		RemindDaysBefore: strconv.Itoa(record.RemindDaysBefore),
	}
	a.renderReminderForm(c, http.StatusOK, "编辑提醒", "/reminders/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) reminderUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/reminders?error=无效提醒ID")
		return
	}

	var existing models.Reminder
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error=提醒不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/reminders?error=无权编辑他人提醒")
		return
	}

	record, formView, bindErr := bindReminderForm(c)
	formView.ID = existing.ID
	if bindErr != nil {
		a.renderReminderForm(c, http.StatusBadRequest, "编辑提醒", "/reminders/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	existing.Title = record.Title
	existing.Content = record.Content
	existing.StartDate = record.StartDate
	existing.EndDate = record.EndDate
	existing.RemindDaysBefore = record.RemindDaysBefore
	existing.UpdatedAt = time.Now()
	if err := a.DB.Save(&existing).Error; err != nil {
		a.renderReminderForm(c, http.StatusBadRequest, "编辑提醒", "/reminders/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/reminders?msg=提醒更新成功")
}

func (a *AppContext) reminderDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/reminders?error=无效提醒ID")
		return
	}

	var existing models.Reminder
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error=提醒不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/reminders?error=无权删除他人提醒")
		return
	}

	if err := a.DB.Delete(&models.Reminder{}, existing.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error="+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/reminders?msg=提醒删除成功")
}

func (a *AppContext) reminderToggleComplete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/reminders?error=无效提醒ID")
		return
	}

	var existing models.Reminder
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error=提醒不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/reminders?error=无权操作他人提醒")
		return
	}

	now := time.Now()
	existing.IsCompleted = !existing.IsCompleted
	if existing.IsCompleted {
		existing.CompletedAt = &now
	} else {
		existing.CompletedAt = nil
	}
	existing.UpdatedAt = now

	if err := a.DB.Save(&existing).Error; err != nil {
		c.Redirect(http.StatusFound, "/reminders?error="+err.Error())
		return
	}

	if existing.IsCompleted {
		c.Redirect(http.StatusFound, "/reminders?msg=提醒已标记完成")
		return
	}
	c.Redirect(http.StatusFound, "/reminders?msg=提醒已恢复为进行中")
}

func bindReminderForm(c *gin.Context) (models.Reminder, reminderFormView, error) {
	form := reminderFormView{
		Title:            strings.TrimSpace(c.PostForm("title")),
		Content:          strings.TrimSpace(c.PostForm("content")),
		StartDate:        strings.TrimSpace(c.PostForm("start_date")),
		EndDate:          strings.TrimSpace(c.PostForm("end_date")),
		RemindDaysBefore: strings.TrimSpace(c.PostForm("remind_days_before")),
	}

	if form.Title == "" {
		return models.Reminder{}, form, fmt.Errorf("标题不能为空")
	}

	startDate, err := parseRequiredDate(form.StartDate)
	if err != nil {
		return models.Reminder{}, form, fmt.Errorf("开始日期无效")
	}
	endDate, err := parseRequiredDate(form.EndDate)
	if err != nil {
		return models.Reminder{}, form, fmt.Errorf("结束日期无效")
	}
	if endDate.Before(startDate) {
		return models.Reminder{}, form, fmt.Errorf("结束日期不能早于开始日期")
	}

	remindDaysBefore := 2
	if form.RemindDaysBefore != "" {
		n, parseErr := strconv.Atoi(form.RemindDaysBefore)
		if parseErr != nil {
			return models.Reminder{}, form, fmt.Errorf("提前提醒天数必须为整数")
		}
		if n < 0 || n > 365 {
			return models.Reminder{}, form, fmt.Errorf("提前提醒天数必须在 0 到 365 之间")
		}
		remindDaysBefore = n
	}

	if form.RemindDaysBefore == "" {
		form.RemindDaysBefore = strconv.Itoa(remindDaysBefore)
	}

	return models.Reminder{
		Title:            form.Title,
		Content:          form.Content,
		StartDate:        startDate,
		EndDate:          endDate,
		RemindDaysBefore: remindDaysBefore,
	}, form, nil
}

func (a *AppContext) renderReminderForm(c *gin.Context, statusCode int, title, action string, form reminderFormView, errorMessage string) {
	if strings.TrimSpace(form.RemindDaysBefore) == "" {
		form.RemindDaysBefore = "2"
	}

	c.HTML(statusCode, "reminder/form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}

func reminderStatusBadge(record models.Reminder, now time.Time) (string, string) {
	if record.IsCompleted {
		return "已完成", "bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-200"
	}
	today := normalizeToDate(now)
	deadline := normalizeToDate(record.EndDate)
	if today.After(deadline) {
		overdueDays := int(today.Sub(deadline).Hours() / 24)
		if overdueDays < 1 {
			overdueDays = 1
		}
		return fmt.Sprintf("已逾期 %d 天", overdueDays), "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200"
	}

	triggerDate := deadline.AddDate(0, 0, -maxInt(record.RemindDaysBefore, 0))
	if !today.Before(triggerDate) {
		remainingDays := int(deadline.Sub(today).Hours() / 24)
		if remainingDays <= 0 {
			return "今日到期", "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200"
		}
		return fmt.Sprintf("%d 天后到期", remainingDays), "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200"
	}

	return "进行中", "bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200"
}

func normalizeToDate(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
