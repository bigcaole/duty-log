package handlers

import "testing"

func TestCollectBulkSystemConfigUpdates(t *testing.T) {
	form := map[string]string{
		"AI_API_KEY":            "",
		"MAIL_PORT":             "587",
		"BACKUP_ENABLED":        "true",
		"backup_schedule_type":  "weekly",
		"backup_weekday":        "0",
		"backup_hour":           "18",
		"backup_minute":         "0",
		"backup_month_day":      "1",
		"backup_month":          "1",
		"BACKUP_SCHEDULE":       "",
		"BACKUP_RETENTION_DAYS": "30",
	}

	updates, needsReload, err := collectBulkSystemConfigUpdates(func(key string) string {
		return form[key]
	})
	if err != nil {
		t.Fatalf("collectBulkSystemConfigUpdates failed: %v", err)
	}
	if !needsReload {
		t.Fatalf("expected backup config changes to require reload")
	}

	hasMailPort := false
	hasAIKey := false
	for _, update := range updates {
		if update.Key == "MAIL_PORT" && update.Value == "587" {
			hasMailPort = true
		}
		if update.Key == "AI_API_KEY" {
			hasAIKey = true
		}
	}
	if !hasMailPort {
		t.Fatalf("expected MAIL_PORT update to exist")
	}
	if hasAIKey {
		t.Fatalf("expected empty sensitive AI_API_KEY to be skipped")
	}
}

func TestCollectBulkSystemConfigUpdatesInvalidValue(t *testing.T) {
	form := map[string]string{
		"MAIL_PORT": "invalid",
	}

	_, _, err := collectBulkSystemConfigUpdates(func(key string) string {
		return form[key]
	})
	if err == nil {
		t.Fatalf("expected invalid MAIL_PORT to fail")
	}
}
