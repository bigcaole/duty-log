package handlers

import (
	"testing"
	"time"

	"duty-log-system/internal/models"
)

func TestReminderStatusBadge(t *testing.T) {
	now := time.Date(2026, 3, 6, 10, 0, 0, 0, time.Local)

	completed := models.Reminder{IsCompleted: true}
	label, _ := reminderStatusBadge(completed, now)
	if label != "已完成" {
		t.Fatalf("unexpected completed label: %s", label)
	}

	dueSoon := models.Reminder{EndDate: now.AddDate(0, 0, 2), RemindDaysBefore: 2}
	label, _ = reminderStatusBadge(dueSoon, now)
	if label != "1 天后到期" {
		t.Fatalf("unexpected due-soon label: %s", label)
	}

	overdue := models.Reminder{EndDate: now.AddDate(0, 0, -1), RemindDaysBefore: 2}
	label, _ = reminderStatusBadge(overdue, now)
	if label != "已逾期 1 天" {
		t.Fatalf("unexpected overdue label: %s", label)
	}
}
