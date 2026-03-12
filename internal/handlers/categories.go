package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

type simpleCategoryForm struct {
	ID          uint
	Name        string
	Description string
}

func registerCategoryRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/categories", app.ticketCategoryList)
	group.GET("/categories/create", app.ticketCategoryCreatePage)
	group.POST("/categories/create", app.ticketCategoryCreate)
	group.GET("/categories/:id/edit", app.ticketCategoryEditPage)
	group.POST("/categories/:id/edit", app.ticketCategoryUpdate)
	group.POST("/categories/:id/delete", app.ticketCategoryDelete)

	registerTypeRoutes(group, app)

	group.POST("/ticket-categories/add", app.ticketCategoryQuickAdd)
	group.POST("/ticket-categories/:id/delete", app.ticketCategoryDelete)
	group.POST("/task-categories/add", app.taskCategoryQuickAdd)
	group.POST("/task-categories/:id/delete", app.taskCategoryDelete)
}

func registerTypeRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/work-ticket-types", app.workTicketTypeList)
	group.GET("/work-ticket-types/create", app.workTicketTypeCreatePage)
	group.POST("/work-ticket-types/create", app.workTicketTypeCreate)
	group.GET("/work-ticket-types/:id/edit", app.workTicketTypeEditPage)
	group.POST("/work-ticket-types/:id/edit", app.workTicketTypeUpdate)
	group.POST("/work-ticket-types/:id/delete", app.workTicketTypeDelete)

	group.GET("/idc-ops-ticket-types", app.idcOpsTicketTypeList)
	group.GET("/idc-ops-ticket-types/create", app.idcOpsTicketTypeCreatePage)
	group.POST("/idc-ops-ticket-types/create", app.idcOpsTicketTypeCreate)
	group.GET("/idc-ops-ticket-types/:id/edit", app.idcOpsTicketTypeEditPage)
	group.POST("/idc-ops-ticket-types/:id/edit", app.idcOpsTicketTypeUpdate)
	group.POST("/idc-ops-ticket-types/:id/delete", app.idcOpsTicketTypeDelete)

	group.GET("/idc-event-types", app.idcEventTypeList)
	group.POST("/idc-event-types/create", app.idcEventTypeCreate)
	group.GET("/idc-event-types/:id/edit", app.idcEventTypeEditPage)
	group.POST("/idc-event-types/:id/edit", app.idcEventTypeUpdate)
	group.POST("/idc-event-types/:id/delete", app.idcEventTypeDelete)

	group.GET("/fault-types", app.faultTypeList)
	group.GET("/fault-types/create", app.faultTypeCreatePage)
	group.POST("/fault-types/create", app.faultTypeCreate)
	group.GET("/fault-types/:id/edit", app.faultTypeEditPage)
	group.POST("/fault-types/:id/edit", app.faultTypeUpdate)
	group.POST("/fault-types/:id/delete", app.faultTypeDelete)
}
func (a *AppContext) ticketCategoryList(c *gin.Context) {
	var ticketCategories []models.TicketCategory
	var taskCategories []models.TaskCategory
	_ = a.DB.Order("name asc").Find(&ticketCategories).Error
	_ = a.DB.Order("name asc").Find(&taskCategories).Error

	c.HTML(http.StatusOK, "admin/categories.html", gin.H{
		"Title":            "分类管理",
		"TicketCategories": ticketCategories,
		"TaskCategories":   taskCategories,
		"Msg":              strings.TrimSpace(c.Query("msg")),
		"Error":            strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) ticketCategoryCreatePage(c *gin.Context) {
	a.renderCategoryForm(c, http.StatusOK, "新建工单分类", "/admin/categories/create", simpleCategoryForm{}, "")
}

func (a *AppContext) ticketCategoryCreate(c *gin.Context) {
	form, err := bindSimpleCategoryForm(c)
	if err != nil {
		a.renderCategoryForm(c, http.StatusBadRequest, "新建工单分类", "/admin/categories/create", form, err.Error())
		return
	}

	record := models.TicketCategory{Name: form.Name, Description: form.Description}
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderCategoryForm(c, http.StatusBadRequest, "新建工单分类", "/admin/categories/create", form, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=创建成功")
}

func (a *AppContext) ticketCategoryEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/categories?error=无效ID")
		return
	}
	var record models.TicketCategory
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error=分类不存在")
		return
	}

	form := simpleCategoryForm{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
	}
	a.renderCategoryForm(c, http.StatusOK, "编辑工单分类", "/admin/categories/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) ticketCategoryUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/categories?error=无效ID")
		return
	}

	var record models.TicketCategory
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error=分类不存在")
		return
	}

	form, bindErr := bindSimpleCategoryForm(c)
	form.ID = record.ID
	if bindErr != nil {
		a.renderCategoryForm(c, http.StatusBadRequest, "编辑工单分类", "/admin/categories/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}

	record.Name = form.Name
	record.Description = form.Description
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderCategoryForm(c, http.StatusBadRequest, "编辑工单分类", "/admin/categories/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=更新成功")
}

func (a *AppContext) ticketCategoryDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/categories?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.TicketCategory{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=删除成功")
}

func (a *AppContext) ticketCategoryQuickAdd(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	if name == "" {
		c.Redirect(http.StatusFound, "/admin/categories?error=名称不能为空")
		return
	}
	if err := a.DB.Create(&models.TicketCategory{Name: name, Description: description}).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=工单分类已添加")
}

func (a *AppContext) taskCategoryQuickAdd(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	if name == "" {
		c.Redirect(http.StatusFound, "/admin/categories?error=IDC事件类型名称不能为空")
		return
	}
	if err := a.DB.Create(&models.TaskCategory{Name: name, Description: description}).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=IDC事件类型已添加")
}

func (a *AppContext) taskCategoryDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/categories?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.TaskCategory{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?msg=IDC事件类型已删除")
}

func (a *AppContext) workTicketTypeList(c *gin.Context) {
	var records []models.WorkTicketType
	_ = a.DB.Order("name asc").Find(&records).Error
	c.HTML(http.StatusOK, "admin/work_ticket_types.html", gin.H{
		"Title": "工单类型管理",
		"Items": records,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) workTicketTypeCreatePage(c *gin.Context) {
	a.renderWorkTicketTypeForm(c, http.StatusOK, "新建工单类型", "/admin/work-ticket-types/create", simpleCategoryForm{}, "")
}

func (a *AppContext) workTicketTypeCreate(c *gin.Context) {
	form, err := bindSimpleCategoryForm(c)
	if err != nil {
		a.renderWorkTicketTypeForm(c, http.StatusBadRequest, "新建工单类型", "/admin/work-ticket-types/create", form, err.Error())
		return
	}
	if err := a.DB.Create(&models.WorkTicketType{Name: form.Name, Description: form.Description}).Error; err != nil {
		a.renderWorkTicketTypeForm(c, http.StatusBadRequest, "新建工单类型", "/admin/work-ticket-types/create", form, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/work-ticket-types?msg=创建成功")
}

func (a *AppContext) workTicketTypeEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error=无效ID")
		return
	}
	var record models.WorkTicketType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error=记录不存在")
		return
	}
	form := simpleCategoryForm{ID: record.ID, Name: record.Name, Description: record.Description}
	a.renderWorkTicketTypeForm(c, http.StatusOK, "编辑工单类型", "/admin/work-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) workTicketTypeUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error=无效ID")
		return
	}
	var record models.WorkTicketType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error=记录不存在")
		return
	}

	form, bindErr := bindSimpleCategoryForm(c)
	form.ID = record.ID
	if bindErr != nil {
		a.renderWorkTicketTypeForm(c, http.StatusBadRequest, "编辑工单类型", "/admin/work-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}
	record.Name = form.Name
	record.Description = form.Description
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderWorkTicketTypeForm(c, http.StatusBadRequest, "编辑工单类型", "/admin/work-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/work-ticket-types?msg=更新成功")
}

func (a *AppContext) workTicketTypeDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.WorkTicketType{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/work-ticket-types?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/work-ticket-types?msg=删除成功")
}

func (a *AppContext) idcOpsTicketTypeList(c *gin.Context) {
	var records []models.IDCOpsTicketType
	_ = a.DB.Order("name asc").Find(&records).Error
	c.HTML(http.StatusOK, "admin/idc_ops_ticket_types.html", gin.H{
		"Title": "IDC 工单类型管理",
		"Items": records,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) idcOpsTicketTypeCreatePage(c *gin.Context) {
	a.renderIDCOpsTicketTypeForm(c, http.StatusOK, "新建 IDC 工单类型", "/admin/idc-ops-ticket-types/create", simpleCategoryForm{}, "")
}

func (a *AppContext) idcOpsTicketTypeCreate(c *gin.Context) {
	form, err := bindSimpleCategoryForm(c)
	if err != nil {
		a.renderIDCOpsTicketTypeForm(c, http.StatusBadRequest, "新建 IDC 工单类型", "/admin/idc-ops-ticket-types/create", form, err.Error())
		return
	}
	if err := a.DB.Create(&models.IDCOpsTicketType{Name: form.Name, Description: form.Description}).Error; err != nil {
		a.renderIDCOpsTicketTypeForm(c, http.StatusBadRequest, "新建 IDC 工单类型", "/admin/idc-ops-ticket-types/create", form, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?msg=创建成功")
}

func (a *AppContext) idcOpsTicketTypeEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error=无效ID")
		return
	}
	var record models.IDCOpsTicketType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error=记录不存在")
		return
	}
	form := simpleCategoryForm{ID: record.ID, Name: record.Name, Description: record.Description}
	a.renderIDCOpsTicketTypeForm(c, http.StatusOK, "编辑 IDC 工单类型", "/admin/idc-ops-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) idcOpsTicketTypeUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error=无效ID")
		return
	}
	var record models.IDCOpsTicketType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error=记录不存在")
		return
	}
	form, bindErr := bindSimpleCategoryForm(c)
	form.ID = record.ID
	if bindErr != nil {
		a.renderIDCOpsTicketTypeForm(c, http.StatusBadRequest, "编辑 IDC 工单类型", "/admin/idc-ops-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}
	record.Name = form.Name
	record.Description = form.Description
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderIDCOpsTicketTypeForm(c, http.StatusBadRequest, "编辑 IDC 工单类型", "/admin/idc-ops-ticket-types/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?msg=更新成功")
}

func (a *AppContext) idcOpsTicketTypeDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.IDCOpsTicketType{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-ops-ticket-types?msg=删除成功")
}

func (a *AppContext) renderIDCOpsTicketTypeForm(c *gin.Context, statusCode int, title, action string, form simpleCategoryForm, errorMessage string) {
	c.HTML(statusCode, "admin/idc_ops_ticket_type_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}

func (a *AppContext) idcEventTypeList(c *gin.Context) {
	var records []models.TaskCategory
	_ = a.DB.Order("name asc").Find(&records).Error
	c.HTML(http.StatusOK, "admin/idc_event_types.html", gin.H{
		"Title": "IDC 事件类型管理",
		"Items": records,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) idcEventTypeCreate(c *gin.Context) {
	form, err := bindSimpleCategoryForm(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error="+err.Error())
		return
	}
	if err := a.DB.Create(&models.TaskCategory{Name: form.Name, Description: form.Description}).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-event-types?msg=创建成功")
}

func (a *AppContext) idcEventTypeEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error=无效ID")
		return
	}
	var record models.TaskCategory
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error=记录不存在")
		return
	}
	form := simpleCategoryForm{ID: record.ID, Name: record.Name, Description: record.Description}
	a.renderIDCEventTypeForm(c, http.StatusOK, "编辑 IDC 事件类型", "/admin/idc-event-types/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) idcEventTypeUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error=无效ID")
		return
	}
	var record models.TaskCategory
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error=记录不存在")
		return
	}
	form, bindErr := bindSimpleCategoryForm(c)
	form.ID = record.ID
	if bindErr != nil {
		a.renderIDCEventTypeForm(c, http.StatusBadRequest, "编辑 IDC 事件类型", "/admin/idc-event-types/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}
	record.Name = form.Name
	record.Description = form.Description
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderIDCEventTypeForm(c, http.StatusBadRequest, "编辑 IDC 事件类型", "/admin/idc-event-types/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-event-types?msg=更新成功")
}

func (a *AppContext) idcEventTypeDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.TaskCategory{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/idc-event-types?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/idc-event-types?msg=删除成功")
}

func (a *AppContext) renderIDCEventTypeForm(c *gin.Context, statusCode int, title, action string, form simpleCategoryForm, errorMessage string) {
	c.HTML(statusCode, "admin/idc_event_type_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}

func (a *AppContext) faultTypeList(c *gin.Context) {
	var records []models.FaultType
	_ = a.DB.Order("name asc").Find(&records).Error
	c.HTML(http.StatusOK, "admin/fault_types.html", gin.H{
		"Title": "故障类型管理",
		"Items": records,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) faultTypeCreatePage(c *gin.Context) {
	a.renderFaultTypeForm(c, http.StatusOK, "新建故障类型", "/admin/fault-types/create", simpleCategoryForm{}, "")
}

func (a *AppContext) faultTypeCreate(c *gin.Context) {
	form, err := bindSimpleCategoryForm(c)
	if err != nil {
		a.renderFaultTypeForm(c, http.StatusBadRequest, "新建故障类型", "/admin/fault-types/create", form, err.Error())
		return
	}
	if err := a.DB.Create(&models.FaultType{Name: form.Name, Description: form.Description}).Error; err != nil {
		a.renderFaultTypeForm(c, http.StatusBadRequest, "新建故障类型", "/admin/fault-types/create", form, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/fault-types?msg=创建成功")
}

func (a *AppContext) faultTypeEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/fault-types?error=无效ID")
		return
	}
	var record models.FaultType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/fault-types?error=记录不存在")
		return
	}
	form := simpleCategoryForm{ID: record.ID, Name: record.Name, Description: record.Description}
	a.renderFaultTypeForm(c, http.StatusOK, "编辑故障类型", "/admin/fault-types/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) faultTypeUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/fault-types?error=无效ID")
		return
	}
	var record models.FaultType
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/fault-types?error=记录不存在")
		return
	}
	form, bindErr := bindSimpleCategoryForm(c)
	form.ID = record.ID
	if bindErr != nil {
		a.renderFaultTypeForm(c, http.StatusBadRequest, "编辑故障类型", "/admin/fault-types/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}
	record.Name = form.Name
	record.Description = form.Description
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderFaultTypeForm(c, http.StatusBadRequest, "编辑故障类型", "/admin/fault-types/"+strconv.FormatUint(id, 10)+"/edit", form, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/fault-types?msg=更新成功")
}

func (a *AppContext) faultTypeDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/fault-types?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.FaultType{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/fault-types?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/fault-types?msg=删除成功")
}

func bindSimpleCategoryForm(c *gin.Context) (simpleCategoryForm, error) {
	form := simpleCategoryForm{
		Name:        strings.TrimSpace(c.PostForm("name")),
		Description: strings.TrimSpace(c.PostForm("description")),
	}
	if form.Name == "" {
		return form, fmt.Errorf("名称不能为空")
	}
	return form, nil
}

func (a *AppContext) renderCategoryForm(c *gin.Context, statusCode int, title, action string, form simpleCategoryForm, errorMessage string) {
	c.HTML(statusCode, "admin/category_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}

func (a *AppContext) renderWorkTicketTypeForm(c *gin.Context, statusCode int, title, action string, form simpleCategoryForm, errorMessage string) {
	c.HTML(statusCode, "admin/work_ticket_type_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}

func (a *AppContext) renderFaultTypeForm(c *gin.Context, statusCode int, title, action string, form simpleCategoryForm, errorMessage string) {
	c.HTML(statusCode, "admin/fault_type_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errorMessage,
	})
}
