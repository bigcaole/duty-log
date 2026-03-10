package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

type reminderRequest struct {
	Enabled    bool
	Date       string
	Title      string
	Content    string
	DaysBefore string
}

func readReminderRequest(c *gin.Context) reminderRequest {
	return reminderRequest{
		Enabled:    parseBoolForm(c, "reminder_enabled"),
		Date:       strings.TrimSpace(c.PostForm("reminder_date")),
		Title:      strings.TrimSpace(c.PostForm("reminder_title")),
		Content:    strings.TrimSpace(c.PostForm("reminder_content")),
		DaysBefore: strings.TrimSpace(c.PostForm("reminder_days_before")),
	}
}

func buildReminderFromRequest(req reminderRequest, recordDate time.Time, userID uint, fallbackTitle, fallbackContent string) (*models.Reminder, error) {
	if !req.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(req.Date) == "" {
		return nil, fmt.Errorf("提醒日期不能为空")
	}
	endDate, err := parseRequiredDate(req.Date)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fallbackTitle
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		content = fallbackContent
	}
	daysBefore := 2
	if strings.TrimSpace(req.DaysBefore) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(req.DaysBefore))
		if err != nil {
			return nil, fmt.Errorf("提前提醒天数必须为整数")
		}
		if parsed < 0 || parsed > 365 {
			return nil, fmt.Errorf("提前提醒天数必须在 0 到 365 之间")
		}
		daysBefore = parsed
	}

	reminder := &models.Reminder{
		UserID:           userID,
		Title:            title,
		Content:          content,
		StartDate:        recordDate,
		EndDate:          endDate,
		RemindDaysBefore: daysBefore,
		IsCompleted:      false,
	}
	return reminder, nil
}
