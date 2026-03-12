package middleware

import (
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AuditLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if db == nil {
			return
		}
		if !shouldAuditMethod(c.Request.Method) {
			return
		}
		if c.Writer.Status() >= 400 {
			return
		}

		var userID *uint
		if uid, ok := CurrentUserID(c); ok && uid > 0 {
			userID = &uid
		}

		path := c.Request.URL.Path
		action := inferAction(c.Request.Method, path)
		tableName := inferTableName(path)
		recordID := inferRecordID(path)

		details := models.JSONMap{
			"method":      c.Request.Method,
			"path":        path,
			"status_code": c.Writer.Status(),
			"duration_ms": time.Since(start).Milliseconds(),
		}

		entry := models.AuditLog{
			UserID:      userID,
			Action:      action,
			TableName:   tableName,
			RecordID:    recordID,
			DetailsJSON: details,
			IPAddress:   strings.TrimSpace(c.ClientIP()),
		}
		_ = db.Create(&entry).Error
	}
}

func shouldAuditMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func inferAction(method, path string) string {
	p := strings.ToLower(strings.TrimSpace(path))
	if strings.Contains(p, "/generate-weekly-summary") {
		return "generate_report"
	}
	if strings.Contains(p, "/backups/run") {
		return "run_backup"
	}
	if strings.Contains(p, "/config") {
		return "config_update"
	}
	if strings.Contains(p, "/delete") || strings.ToUpper(method) == "DELETE" {
		return "delete"
	}
	if strings.Contains(p, "/edit") || strings.Contains(p, "/update") {
		return "update"
	}
	if strings.Contains(p, "/create") || strings.Contains(p, "/add") || strings.ToUpper(method) == "POST" {
		return "create"
	}
	return "request"
}

func inferTableName(path string) string {
	parts := splitPath(path)
	if len(parts) == 0 {
		return ""
	}

	seg := parts[0]
	if seg == "admin" && len(parts) > 1 {
		seg = parts[1]
	}

	switch seg {
	case "idc-ops-tickets":
		return "idc_ops_tickets"
	case "tickets":
		return "tickets"
	case "duty-logs":
		return "duty_logs"
	case "idc-duty":
		return "idc_duty_records"
	case "work-tickets":
		return "work_tickets"
	case "fault-records":
		return "fault_records"
	case "instructions":
		return "instructions"
	case "users":
		return "users"
	case "categories", "ticket-categories":
		return "ticket_categories"
	case "task-categories":
		return "task_categories"
	case "work-ticket-types":
		return "work_ticket_types"
	case "idc-ops-ticket-types":
		return "idc_ops_ticket_types"
	case "fault-types":
		return "fault_types"
	case "config":
		return "system_configs"
	case "backup-notifications", "backups":
		return "backup_notifications"
	default:
		return seg
	}
}

func inferRecordID(path string) *uint {
	for _, part := range splitPath(path) {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || n == 0 {
			continue
		}
		id := uint(n)
		return &id
	}
	return nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
