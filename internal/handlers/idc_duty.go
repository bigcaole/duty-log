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

type idcDutyListItem struct {
	ID           uint
	Date         string
	NewDate      bool
	DutyIdc      string
	TaskCategory string
	Tasks        string
	UpdatedAt    string
}

type idcDutyFormView struct {
	ID             uint
	Date           string
	DutyIdc        string
	TaskCategoryID string
	Tasks          string
}

func registerIDCDutyRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/idc-duty", app.idcDutyList)
	group.GET("/idc-duty/create", app.idcDutyCreatePage)
	group.POST("/idc-duty", app.idcDutyCreate)
	group.GET("/idc-duty/:id", app.idcDutyDetail)
	group.GET("/idc-duty/:id/edit", app.idcDutyEditPage)
	group.POST("/idc-duty/:id", app.idcDutyUpdate)
	group.POST("/idc-duty/:id/delete", app.idcDutyDelete)
}

func (a *AppContext) idcDutyList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var records []models.IdcDutyRecord
	query := a.DB.Order("date desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "IDC 值班记录",
			"Path":    "/idc-duty",
			"Message": "读取值班记录失败：" + err.Error(),
		})
		return
	}

	var categories []models.TaskCategory
	_ = a.DB.Order("name asc").Find(&categories).Error
	categoryNameByID := make(map[uint]string, len(categories))
	for _, item := range categories {
		categoryNameByID[item.ID] = item.Name
	}

	items := make([]idcDutyListItem, 0, len(records))
	lastDate := ""
	for _, record := range records {
		categoryName := "-"
		if record.TaskCategoryID != nil {
			if name, ok := categoryNameByID[*record.TaskCategoryID]; ok {
				categoryName = name
			}
		}
		dateText := record.Date.Format(dateLayout)
		newDate := dateText != lastDate
		if newDate {
			lastDate = dateText
		}
		items = append(items, idcDutyListItem{
			ID:           record.ID,
			Date:         dateText,
			NewDate:      newDate,
			DutyIdc:      record.DutyIdc,
			TaskCategory: categoryName,
			Tasks:        trimDashboardText(record.Tasks, 80),
			UpdatedAt:    record.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "idc_duty/list.html", gin.H{
		"Title":   "IDC 值班记录",
		"Items":   items,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) idcDutyCreatePage(c *gin.Context) {
	a.renderIDCDutyForm(c, http.StatusOK, "新增 IDC 值班记录", "/idc-duty", idcDutyFormView{
		Date: todayDateString(),
	}, "")
}

func (a *AppContext) idcDutyCreate(c *gin.Context) {
	record, formView, err := a.bindIDCDutyForm(c)
	if err != nil {
		a.renderIDCDutyForm(c, http.StatusBadRequest, "新增 IDC 值班记录", "/idc-duty", formView, err.Error())
		return
	}
	userID, ok := middleware.CurrentUserID(c)
	if !ok || userID == 0 {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	record.UserID = &userID
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderIDCDutyForm(c, http.StatusBadRequest, "新增 IDC 值班记录", "/idc-duty", formView, idcDutyWriteErrorMessage("保存失败：", err))
		return
	}
	c.Redirect(http.StatusFound, "/idc-duty?msg=创建成功")
}

func (a *AppContext) idcDutyDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-duty?error=无效记录ID")
		return
	}

	var record models.IdcDutyRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-duty?error=记录不存在")
		return
	}
	if record.UserID == nil {
		if !currentUser.IsAdmin {
			c.Redirect(http.StatusFound, "/idc-duty?error=无权查看该记录")
			return
		}
	} else if !canAccessOwnedRecord(currentUser.IsAdmin, *record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-duty?error=无权查看他人记录")
		return
	}

	categoryName := "-"
	if record.TaskCategoryID != nil {
		var cat models.TaskCategory
		if err := a.DB.First(&cat, *record.TaskCategoryID).Error; err == nil {
			categoryName = cat.Name
		}
	}

	c.HTML(http.StatusOK, "idc_duty/detail.html", gin.H{
		"Title":        "IDC值班记录预览",
		"Record":       record,
		"CategoryName": categoryName,
	})
}

func (a *AppContext) idcDutyEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-duty?error=无效记录ID")
		return
	}

	var record models.IdcDutyRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-duty?error=记录不存在")
		return
	}
	if record.UserID == nil {
		if !currentUser.IsAdmin {
			c.Redirect(http.StatusFound, "/idc-duty?error=无权编辑该记录")
			return
		}
	} else if !canAccessOwnedRecord(currentUser.IsAdmin, *record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-duty?error=无权编辑他人记录")
		return
	}

	formView := idcDutyFormView{
		ID:      record.ID,
		Date:    record.Date.Format(dateLayout),
		DutyIdc: record.DutyIdc,
		Tasks:   record.Tasks,
	}
	if record.TaskCategoryID != nil {
		formView.TaskCategoryID = strconv.FormatUint(uint64(*record.TaskCategoryID), 10)
	}

	a.renderIDCDutyForm(c, http.StatusOK, "编辑 IDC 值班记录", "/idc-duty/"+strconv.FormatUint(id, 10), formView, "")
}

func (a *AppContext) idcDutyUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-duty?error=无效记录ID")
		return
	}

	var existing models.IdcDutyRecord
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-duty?error=记录不存在")
		return
	}
	if existing.UserID == nil {
		if !currentUser.IsAdmin {
			c.Redirect(http.StatusFound, "/idc-duty?error=无权编辑该记录")
			return
		}
	} else if !canAccessOwnedRecord(currentUser.IsAdmin, *existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-duty?error=无权编辑他人记录")
		return
	}

	record, formView, bindErr := a.bindIDCDutyForm(c)
	formView.ID = existing.ID
	if bindErr != nil {
		a.renderIDCDutyForm(c, http.StatusBadRequest, "编辑 IDC 值班记录", "/idc-duty/"+strconv.FormatUint(id, 10), formView, bindErr.Error())
		return
	}

	existing.Date = record.Date
	existing.DutyIdc = record.DutyIdc
	existing.TaskCategoryID = record.TaskCategoryID
	existing.Tasks = record.Tasks
	existing.UpdatedAt = time.Now()

	if err := a.DB.Save(&existing).Error; err != nil {
		a.renderIDCDutyForm(c, http.StatusBadRequest, "编辑 IDC 值班记录", "/idc-duty/"+strconv.FormatUint(id, 10), formView, idcDutyWriteErrorMessage("更新失败：", err))
		return
	}
	c.Redirect(http.StatusFound, "/idc-duty?msg=更新成功")
}

func (a *AppContext) idcDutyDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/idc-duty?error=无效记录ID")
		return
	}
	var record models.IdcDutyRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-duty?error=记录不存在")
		return
	}
	if record.UserID == nil {
		if !currentUser.IsAdmin {
			c.Redirect(http.StatusFound, "/idc-duty?error=无权删除该记录")
			return
		}
	} else if !canAccessOwnedRecord(currentUser.IsAdmin, *record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/idc-duty?error=无权删除他人记录")
		return
	}
	if err := a.DB.Delete(&models.IdcDutyRecord{}, record.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/idc-duty?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/idc-duty?msg=删除成功")
}

func (a *AppContext) bindIDCDutyForm(c *gin.Context) (models.IdcDutyRecord, idcDutyFormView, error) {
	formView := idcDutyFormView{
		Date:           strings.TrimSpace(c.PostForm("date")),
		DutyIdc:        strings.TrimSpace(c.PostForm("duty_idc")),
		TaskCategoryID: strings.TrimSpace(c.PostForm("task_category_id")),
		Tasks:          strings.TrimSpace(c.PostForm("tasks")),
	}

	date, err := parseRequiredDate(formView.Date)
	if err != nil {
		return models.IdcDutyRecord{}, formView, err
	}
	if formView.DutyIdc == "" {
		return models.IdcDutyRecord{}, formView, fmt.Errorf("IDC值班人员不能为空")
	}

	categoryID, err := parseOptionalUint(formView.TaskCategoryID)
	if err != nil {
		return models.IdcDutyRecord{}, formView, err
	}

	record := models.IdcDutyRecord{
		Date:           date,
		DutyOps:        "系统默认",
		DutyIdc:        formView.DutyIdc,
		TaskCategoryID: categoryID,
		Tasks:          formView.Tasks,
		VisitsJSON:     models.JSONSlice{},
	}
	return record, formView, nil
}

func (a *AppContext) renderIDCDutyForm(c *gin.Context, statusCode int, title, action string, formView idcDutyFormView, errorMessage string) {
	var categories []models.TaskCategory
	_ = a.DB.Order("name asc").Find(&categories).Error

	c.HTML(statusCode, "idc_duty/form.html", gin.H{
		"Title":      title,
		"Action":     action,
		"Form":       formView,
		"Categories": categories,
		"Error":      errorMessage,
	})
}

func idcDutyWriteErrorMessage(prefix string, err error) string {
	if isIDCDutyDuplicateDateError(err) {
		return "同一用户在同一天只能创建一条 IDC 值班记录"
	}
	return prefix + err.Error()
}

func isIDCDutyDuplicateDateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "duplicate key value") && !strings.Contains(msg, "unique constraint") {
		return false
	}
	if strings.Contains(msg, "idx_idc_duty_user_date") {
		return true
	}
	if strings.Contains(msg, "idc_duty_records_date_key") {
		return true
	}
	if strings.Contains(msg, "idx_idc_duty_records_date") {
		return true
	}
	return strings.Contains(msg, "idc_duty_records_user_id_date_key")
}
