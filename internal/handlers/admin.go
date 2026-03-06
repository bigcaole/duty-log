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
	"golang.org/x/crypto/bcrypt"
)

type adminUserListItem struct {
	ID        uint
	Username  string
	Email     string
	IsActive  bool
	IsAdmin   bool
	Has2FA    bool
	CreatedAt string
}

type adminUserFormView struct {
	ID       uint
	Username string
	Email    string
	Password string
	IsActive bool
	IsAdmin  bool
}

func registerAdminRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("", app.adminIndex)

	group.GET("/users", app.adminUserList)
	group.GET("/users/create", app.adminUserCreatePage)
	group.POST("/users/create", app.adminUserCreate)
	group.GET("/users/:id/edit", app.adminUserEditPage)
	group.POST("/users/:id/edit", app.adminUserUpdate)
	group.POST("/users/:id/delete", app.adminUserDelete)

	group.POST("/generate-weekly-summary", app.adminGenerateWeeklySummary)
	group.POST("/reports/test-delivery", app.adminTestWeeklyDelivery)
	group.GET("/download-pdf", app.adminDownloadPDF)
	group.GET("/audit-logs", app.adminAuditLogList)
	group.POST("/audit-logs/cleanup", app.adminCleanupAuditLogs)
	group.GET("/backup-notifications", app.adminBackupNotificationList)
	group.POST("/backups/run", app.adminRunBackupNow)
	group.POST("/backup-notifications/cleanup", app.adminCleanupBackupRecords)
	group.POST("/backup-notifications/normalize-passwords", app.adminNormalizeBackupPasswords)
	group.GET("/backup-notifications/:id/download", app.adminDownloadBackupFile)
	group.POST("/backup-notifications/:id/decrypt-download-auto", app.adminDecryptDownloadBackupAuto)
	group.POST("/backup-notifications/:id/decrypt-download", app.adminDecryptDownloadBackup)
}

func (a *AppContext) adminIndex(c *gin.Context) {
	var userCount int64
	var idcCount int64
	var workTicketCount int64
	var faultCount int64
	var auditCount int64
	var backupCount int64
	var reminderCount int64
	_ = a.DB.Model(&models.User{}).Count(&userCount).Error
	_ = a.DB.Model(&models.IdcDutyRecord{}).Count(&idcCount).Error
	_ = a.DB.Model(&models.WorkTicket{}).Count(&workTicketCount).Error
	_ = a.DB.Model(&models.FaultRecord{}).Count(&faultCount).Error
	_ = a.DB.Model(&models.AuditLog{}).Count(&auditCount).Error
	_ = a.DB.Model(&models.BackupNotification{}).Count(&backupCount).Error
	_ = a.DB.Model(&models.Reminder{}).Count(&reminderCount).Error

	c.HTML(http.StatusOK, "admin/index.html", gin.H{
		"Title":           "管理后台",
		"UserCount":       userCount,
		"IDCDutyCount":    idcCount,
		"WorkTicketCount": workTicketCount,
		"FaultCount":      faultCount,
		"AuditCount":      auditCount,
		"BackupCount":     backupCount,
		"ReminderCount":   reminderCount,
	})
}

func (a *AppContext) adminUserList(c *gin.Context) {
	var users []models.User
	if err := a.DB.Order("created_at desc").Find(&users).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "用户管理",
			"Path":    "/admin/users",
			"Message": "读取用户失败：" + err.Error(),
		})
		return
	}

	items := make([]adminUserListItem, 0, len(users))
	for _, user := range users {
		items = append(items, adminUserListItem{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			IsActive:  user.IsActive,
			IsAdmin:   user.IsAdmin,
			Has2FA:    strings.TrimSpace(user.OTPSecret) != "",
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "admin/users.html", gin.H{
		"Title": "用户管理",
		"Items": items,
		"Msg":   strings.TrimSpace(c.Query("msg")),
		"Error": strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) adminUserCreatePage(c *gin.Context) {
	a.renderAdminUserForm(c, http.StatusOK, "新建用户", "/admin/users/create", adminUserFormView{
		IsActive: true,
		IsAdmin:  false,
	}, "")
}

func (a *AppContext) adminUserCreate(c *gin.Context) {
	formView, err := bindAdminUserForm(c, true)
	if err != nil {
		a.renderAdminUserForm(c, http.StatusBadRequest, "新建用户", "/admin/users/create", formView, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(formView.Password), bcrypt.DefaultCost)
	if err != nil {
		a.renderAdminUserForm(c, http.StatusInternalServerError, "新建用户", "/admin/users/create", formView, "密码加密失败")
		return
	}

	user := models.User{
		Username:     formView.Username,
		PasswordHash: string(hash),
		Email:        formView.Email,
		IsActive:     formView.IsActive,
		IsAdmin:      formView.IsAdmin,
	}
	if err := a.DB.Create(&user).Error; err != nil {
		a.renderAdminUserForm(c, http.StatusBadRequest, "新建用户", "/admin/users/create", formView, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=创建成功")
}

func (a *AppContext) adminUserEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/users?error=无效用户ID")
		return
	}

	var user models.User
	if err := a.DB.First(&user, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error=用户不存在")
		return
	}

	formView := adminUserFormView{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		IsActive: user.IsActive,
		IsAdmin:  user.IsAdmin,
	}
	a.renderAdminUserForm(c, http.StatusOK, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) adminUserUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/users?error=无效用户ID")
		return
	}

	var user models.User
	if err := a.DB.First(&user, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error=用户不存在")
		return
	}

	formView, bindErr := bindAdminUserForm(c, false)
	formView.ID = user.ID
	if bindErr != nil {
		a.renderAdminUserForm(c, http.StatusBadRequest, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	if currentUserID, ok := middleware.CurrentUserID(c); ok && currentUserID == user.ID {
		if !formView.IsActive {
			a.renderAdminUserForm(c, http.StatusBadRequest, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, "不能停用当前登录账号")
			return
		}
		if !formView.IsAdmin {
			a.renderAdminUserForm(c, http.StatusBadRequest, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, "不能取消当前登录账号的管理员权限")
			return
		}
	}

	user.Username = formView.Username
	user.Email = formView.Email
	user.IsActive = formView.IsActive
	user.IsAdmin = formView.IsAdmin
	if strings.TrimSpace(formView.Password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(formView.Password), bcrypt.DefaultCost)
		if err != nil {
			a.renderAdminUserForm(c, http.StatusInternalServerError, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, "密码加密失败")
			return
		}
		user.PasswordHash = string(hash)
	}
	if parseBoolForm(c, "reset_2fa") {
		user.OTPSecret = ""
	}
	user.UpdatedAt = time.Now()

	if err := a.DB.Save(&user).Error; err != nil {
		a.renderAdminUserForm(c, http.StatusBadRequest, "编辑用户", "/admin/users/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=更新成功")
}

func (a *AppContext) adminUserDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/admin/users?error=无效用户ID")
		return
	}

	var user models.User
	if err := a.DB.First(&user, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error=用户不存在")
		return
	}

	if currentUserID, ok := middleware.CurrentUserID(c); ok && currentUserID == user.ID {
		c.Redirect(http.StatusFound, "/admin/users?error=不能删除当前登录账号")
		return
	}

	if user.IsAdmin {
		var adminCount int64
		_ = a.DB.Model(&models.User{}).Where("is_admin = ?", true).Count(&adminCount).Error
		if adminCount <= 1 {
			c.Redirect(http.StatusFound, "/admin/users?error=至少需要保留一个管理员")
			return
		}
	}

	if err := a.DB.Delete(&models.User{}, user.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=删除成功")
}

func bindAdminUserForm(c *gin.Context, requirePassword bool) (adminUserFormView, error) {
	formView := adminUserFormView{
		Username: strings.TrimSpace(c.PostForm("username")),
		Email:    strings.TrimSpace(c.PostForm("email")),
		Password: strings.TrimSpace(c.PostForm("password")),
		IsActive: parseBoolForm(c, "is_active"),
		IsAdmin:  parseBoolForm(c, "is_admin"),
	}

	if formView.Username == "" {
		return formView, fmt.Errorf("用户名不能为空")
	}
	if formView.Email == "" {
		return formView, fmt.Errorf("邮箱不能为空")
	}
	if requirePassword && formView.Password == "" {
		return formView, fmt.Errorf("密码不能为空")
	}
	if strings.TrimSpace(formView.Password) != "" && len(formView.Password) < 6 {
		return formView, fmt.Errorf("密码长度不能小于6")
	}
	return formView, nil
}

func (a *AppContext) renderAdminUserForm(c *gin.Context, statusCode int, title, action string, formView adminUserFormView, errorMessage string) {
	c.HTML(statusCode, "admin/user_form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   formView,
		"Error":  errorMessage,
	})
}
