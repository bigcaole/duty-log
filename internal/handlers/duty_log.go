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

type dutyLogListItem struct {
	ID        uint
	Date      string
	Content   string
	Creator   string
	UpdatedAt string
}

type dutyLogFormView struct {
	ID      uint
	Date    string
	Content string
}

func registerDutyLogRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/duty-logs", app.dutyLogList)
	group.GET("/duty-logs/create", app.dutyLogCreatePage)
	group.POST("/duty-logs/create", app.dutyLogCreate)
	group.GET("/duty-logs/:id/edit", app.dutyLogEditPage)
	group.POST("/duty-logs/:id/edit", app.dutyLogUpdate)
	group.POST("/duty-logs/:id/delete", app.dutyLogDelete)
}

func (a *AppContext) dutyLogList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var records []models.DutyLog
	query := a.DB.Order("date desc, id desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "值班日志",
			"Path":    "/duty-logs",
			"Message": "读取值班日志失败：" + err.Error(),
		})
		return
	}

	userIDs := make([]uint, 0, len(records))
	userIDSet := make(map[uint]struct{})
	for _, item := range records {
		if item.UserID == nil {
			continue
		}
		if _, ok := userIDSet[*item.UserID]; !ok {
			userIDs = append(userIDs, *item.UserID)
			userIDSet[*item.UserID] = struct{}{}
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

	items := make([]dutyLogListItem, 0, len(records))
	for _, item := range records {
		creator := "-"
		if item.UserID != nil {
			if username, ok := usernameByID[*item.UserID]; ok {
				creator = username
			}
		}
		items = append(items, dutyLogListItem{
			ID:        item.ID,
			Date:      item.Date.Format(dateLayout),
			Content:   item.Content,
			Creator:   creator,
			UpdatedAt: item.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "duty_log/list.html", gin.H{
		"Title":   "值班日志",
		"Items":   items,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) dutyLogCreatePage(c *gin.Context) {
	a.renderDutyLogForm(c, http.StatusOK, "新建值班日志", "/duty-logs/create", dutyLogFormView{
		Date: todayDateString(),
	}, "")
}

func (a *AppContext) dutyLogCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	record, formView, bindErr := bindDutyLogForm(c)
	if bindErr != nil {
		a.renderDutyLogForm(c, http.StatusBadRequest, "新建值班日志", "/duty-logs/create", formView, bindErr.Error())
		return
	}

	record.UserID = &currentUser.ID
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderDutyLogForm(c, http.StatusBadRequest, "新建值班日志", "/duty-logs/create", formView, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/duty-logs?msg=创建成功")
}

func (a *AppContext) dutyLogEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/duty-logs?error=无效日志ID")
		return
	}

	var record models.DutyLog
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/duty-logs?error=日志不存在")
		return
	}
	if !currentUser.IsAdmin && (record.UserID == nil || *record.UserID != currentUser.ID) {
		c.Redirect(http.StatusFound, "/duty-logs?error=无权编辑他人日志")
		return
	}

	formView := dutyLogFormView{
		ID:      record.ID,
		Date:    record.Date.Format(dateLayout),
		Content: record.Content,
	}
	a.renderDutyLogForm(c, http.StatusOK, "编辑值班日志", "/duty-logs/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) dutyLogUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/duty-logs?error=无效日志ID")
		return
	}

	var record models.DutyLog
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/duty-logs?error=日志不存在")
		return
	}
	if !currentUser.IsAdmin && (record.UserID == nil || *record.UserID != currentUser.ID) {
		c.Redirect(http.StatusFound, "/duty-logs?error=无权编辑他人日志")
		return
	}

	incoming, formView, bindErr := bindDutyLogForm(c)
	formView.ID = record.ID
	if bindErr != nil {
		a.renderDutyLogForm(c, http.StatusBadRequest, "编辑值班日志", "/duty-logs/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	record.Date = incoming.Date
	record.Content = incoming.Content
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderDutyLogForm(c, http.StatusBadRequest, "编辑值班日志", "/duty-logs/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/duty-logs?msg=更新成功")
}

func (a *AppContext) dutyLogDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/duty-logs?error=无效日志ID")
		return
	}

	var record models.DutyLog
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/duty-logs?error=日志不存在")
		return
	}
	if !currentUser.IsAdmin && (record.UserID == nil || *record.UserID != currentUser.ID) {
		c.Redirect(http.StatusFound, "/duty-logs?error=无权删除他人日志")
		return
	}

	if err := a.DB.Delete(&models.DutyLog{}, record.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/duty-logs?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/duty-logs?msg=删除成功")
}

func bindDutyLogForm(c *gin.Context) (models.DutyLog, dutyLogFormView, error) {
	formView := dutyLogFormView{
		Date:    strings.TrimSpace(c.PostForm("date")),
		Content: strings.TrimSpace(c.PostForm("content")),
	}
	date, err := parseRequiredDate(formView.Date)
	if err != nil {
		return models.DutyLog{}, formView, err
	}
	if formView.Content == "" {
		return models.DutyLog{}, formView, fmt.Errorf("日志内容不能为空")
	}

	return models.DutyLog{
		Date:    date,
		Content: formView.Content,
	}, formView, nil
}

func (a *AppContext) renderDutyLogForm(c *gin.Context, statusCode int, title, action string, formView dutyLogFormView, errorMessage string) {
	c.HTML(statusCode, "duty_log/form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   formView,
		"Error":  errorMessage,
	})
}
