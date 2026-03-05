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

type instructionFormView struct {
	ID      uint
	Title   string
	Content string
}

func registerInstructionRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/instructions", app.instructionList)
	group.GET("/instructions/create", app.instructionCreatePage)
	group.POST("/instructions/create", app.instructionCreate)
	group.GET("/instructions/:id/edit", app.instructionEditPage)
	group.POST("/instructions/:id/edit", app.instructionUpdate)
	group.POST("/instructions/:id/delete", app.instructionDelete)
}

func (a *AppContext) instructionList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	var records []models.Instruction
	if err := a.DB.Order("updated_at desc").Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "操作说明",
			"Path":    "/instructions",
			"Message": "读取操作说明失败：" + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "instruction/list.html", gin.H{
		"Title":     "操作说明",
		"Items":     records,
		"CanManage": currentUser.IsAdmin,
		"Msg":       strings.TrimSpace(c.Query("msg")),
		"Error":     strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) instructionCreatePage(c *gin.Context) {
	if !a.ensureAdmin(c) {
		return
	}
	a.renderInstructionForm(c, http.StatusOK, "新建操作说明", "/instructions/create", instructionFormView{}, "")
}

func (a *AppContext) instructionCreate(c *gin.Context) {
	if !a.ensureAdmin(c) {
		return
	}

	record, formView, err := bindInstructionForm(c)
	if err != nil {
		a.renderInstructionForm(c, http.StatusBadRequest, "新建操作说明", "/instructions/create", formView, err.Error())
		return
	}
	if err := a.DB.Create(&record).Error; err != nil {
		a.renderInstructionForm(c, http.StatusBadRequest, "新建操作说明", "/instructions/create", formView, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/instructions?msg=创建成功")
}

func (a *AppContext) instructionEditPage(c *gin.Context) {
	if !a.ensureAdmin(c) {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/instructions?error=无效ID")
		return
	}
	var record models.Instruction
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/instructions?error=记录不存在")
		return
	}

	formView := instructionFormView{
		ID:      record.ID,
		Title:   record.Title,
		Content: record.Content,
	}
	a.renderInstructionForm(c, http.StatusOK, "编辑操作说明", "/instructions/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) instructionUpdate(c *gin.Context) {
	if !a.ensureAdmin(c) {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/instructions?error=无效ID")
		return
	}

	var record models.Instruction
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/instructions?error=记录不存在")
		return
	}

	incoming, formView, bindErr := bindInstructionForm(c)
	formView.ID = record.ID
	if bindErr != nil {
		a.renderInstructionForm(c, http.StatusBadRequest, "编辑操作说明", "/instructions/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	record.Title = incoming.Title
	record.Content = incoming.Content
	record.UpdatedAt = time.Now()
	if err := a.DB.Save(&record).Error; err != nil {
		a.renderInstructionForm(c, http.StatusBadRequest, "编辑操作说明", "/instructions/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/instructions?msg=更新成功")
}

func (a *AppContext) instructionDelete(c *gin.Context) {
	if !a.ensureAdmin(c) {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/instructions?error=无效ID")
		return
	}
	if err := a.DB.Delete(&models.Instruction{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/instructions?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/instructions?msg=删除成功")
}

func (a *AppContext) ensureAdmin(c *gin.Context) bool {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return false
	}
	if !currentUser.IsAdmin {
		c.HTML(http.StatusForbidden, "coming_soon.html", gin.H{
			"Title":   "无权限",
			"Path":    c.Request.URL.Path,
			"Message": "该功能仅管理员可操作。",
		})
		return false
	}
	return true
}

func bindInstructionForm(c *gin.Context) (models.Instruction, instructionFormView, error) {
	formView := instructionFormView{
		Title:   strings.TrimSpace(c.PostForm("title")),
		Content: strings.TrimSpace(c.PostForm("content")),
	}
	if formView.Title == "" {
		return models.Instruction{}, formView, fmt.Errorf("标题不能为空")
	}
	if formView.Content == "" {
		return models.Instruction{}, formView, fmt.Errorf("内容不能为空")
	}
	return models.Instruction{
		Title:   formView.Title,
		Content: formView.Content,
	}, formView, nil
}

func (a *AppContext) renderInstructionForm(c *gin.Context, statusCode int, title, action string, formView instructionFormView, errorMessage string) {
	c.HTML(statusCode, "instruction/form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   formView,
		"Error":  errorMessage,
	})
}
