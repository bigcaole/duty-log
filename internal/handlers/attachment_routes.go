package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

func registerAttachmentRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/attachments/:id", app.downloadAttachment)
	group.POST("/attachments/:id/delete", app.deleteAttachment)
}

func (a *AppContext) downloadAttachment(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	var attachment models.Attachment
	if err := a.DB.First(&attachment, uint(id)).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	if !a.canAccessAttachment(currentUser, attachment) {
		c.Status(http.StatusForbidden)
		return
	}

	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(attachment.Data)
	}
	name := strings.TrimSpace(attachment.Name)
	if name == "" {
		name = fmt.Sprintf("attachment-%d", attachment.ID)
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	c.Data(http.StatusOK, contentType, attachment.Data)
}

func (a *AppContext) deleteAttachment(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid id"})
		return
	}

	var attachment models.Attachment
	if err := a.DB.First(&attachment, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "not found"})
		return
	}

	if !a.canAccessAttachment(currentUser, attachment) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "forbidden"})
		return
	}

	if err := a.DB.Delete(&attachment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *AppContext) canAccessAttachment(user *models.User, attachment models.Attachment) bool {
	if user == nil {
		return false
	}
	if user.IsAdmin {
		return true
	}
	switch strings.TrimSpace(attachment.Module) {
	case "work_ticket":
		var record models.WorkTicket
		if err := a.DB.Select("id", "user_id").First(&record, attachment.ModuleID).Error; err != nil {
			return false
		}
		return record.UserID == user.ID
	case "idc_ops_ticket":
		var record models.IDCOpsTicket
		if err := a.DB.Select("id", "user_id").First(&record, attachment.ModuleID).Error; err != nil {
			return false
		}
		return record.UserID == user.ID
	case "fault_record":
		var record models.FaultRecord
		if err := a.DB.Select("id", "user_id").First(&record, attachment.ModuleID).Error; err != nil {
			return false
		}
		return record.UserID == user.ID
	default:
		return false
	}
}
