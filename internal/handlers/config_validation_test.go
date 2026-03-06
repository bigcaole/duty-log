package handlers

import "testing"

func TestNormalizeSystemConfigValueBool(t *testing.T) {
	got, err := normalizeSystemConfigValue("BACKUP_ENABLED", "TrUe")
	if err != nil {
		t.Fatalf("normalize bool failed: %v", err)
	}
	if got != "true" {
		t.Fatalf("unexpected bool value: %q", got)
	}

	got, err = normalizeSystemConfigValue("MAIL_USE_TLS", "0")
	if err != nil {
		t.Fatalf("normalize bool 0 failed: %v", err)
	}
	if got != "false" {
		t.Fatalf("unexpected bool value for 0: %q", got)
	}

	got, err = normalizeSystemConfigValue("WEEKLY_REPORT_ENABLED", "1")
	if err != nil {
		t.Fatalf("normalize weekly bool failed: %v", err)
	}
	if got != "true" {
		t.Fatalf("unexpected weekly bool value: %q", got)
	}

	if _, err := normalizeSystemConfigValue("BACKUP_ENABLED", "abc"); err == nil {
		t.Fatalf("expected invalid bool to fail")
	}
}

func TestNormalizeSystemConfigValueInt(t *testing.T) {
	cases := []struct {
		key   string
		value string
		want  string
	}{
		{"MAIL_PORT", "587", "587"},
		{"BACKUP_HOUR", "23", "23"},
		{"BACKUP_MINUTE", "0", "0"},
		{"BACKUP_RETENTION_DAYS", "30", "30"},
		{"LOGIN_MAX_ATTEMPTS", "5", "5"},
		{"TOTP_VERIFY_MAX_ATTEMPTS", "6", "6"},
		{"TOTP_VERIFY_BLOCK_SECONDS", "300", "300"},
	}

	for _, tc := range cases {
		got, err := normalizeSystemConfigValue(tc.key, tc.value)
		if err != nil {
			t.Fatalf("%s normalize failed: %v", tc.key, err)
		}
		if got != tc.want {
			t.Fatalf("%s normalize got %q want %q", tc.key, got, tc.want)
		}
	}

	if _, err := normalizeSystemConfigValue("BACKUP_HOUR", "24"); err == nil {
		t.Fatalf("expected BACKUP_HOUR=24 to fail")
	}
	if _, err := normalizeSystemConfigValue("MAIL_PORT", "abc"); err == nil {
		t.Fatalf("expected MAIL_PORT=abc to fail")
	}
	if _, err := normalizeSystemConfigValue("TOTP_VERIFY_MAX_ATTEMPTS", "0"); err == nil {
		t.Fatalf("expected TOTP_VERIFY_MAX_ATTEMPTS=0 to fail")
	}
}

func TestNormalizeSystemConfigValueCron(t *testing.T) {
	got, err := normalizeSystemConfigValue("BACKUP_SCHEDULE", " 0  2 * * * ")
	if err != nil {
		t.Fatalf("normalize cron failed: %v", err)
	}
	if got != "0 2 * * *" {
		t.Fatalf("unexpected cron value: %q", got)
	}

	if _, err := normalizeSystemConfigValue("BACKUP_SCHEDULE", "invalid"); err == nil {
		t.Fatalf("expected invalid cron to fail")
	}
	if _, err := normalizeSystemConfigValue("WEEKLY_REPORT_SCHEDULE", "invalid"); err == nil {
		t.Fatalf("expected invalid weekly cron to fail")
	}
}

func TestNormalizeSystemConfigValueEmailAndURL(t *testing.T) {
	email, err := normalizeSystemConfigValue("BACKUP_EMAIL", " test@example.com ")
	if err != nil {
		t.Fatalf("normalize email failed: %v", err)
	}
	if email != "test@example.com" {
		t.Fatalf("unexpected email value: %q", email)
	}
	if _, err := normalizeSystemConfigValue("BACKUP_EMAIL", "not-an-email"); err == nil {
		t.Fatalf("expected invalid email to fail")
	}

	emailList, err := normalizeSystemConfigValue("WEEKLY_REPORT_EMAIL_TO", " a@example.com, b@example.com ")
	if err != nil {
		t.Fatalf("normalize email list failed: %v", err)
	}
	if emailList != "a@example.com,b@example.com" {
		t.Fatalf("unexpected email list value: %q", emailList)
	}
	if _, err := normalizeSystemConfigValue("WEEKLY_REPORT_EMAIL_TO", "a@example.com,invalid"); err == nil {
		t.Fatalf("expected invalid weekly email list to fail")
	}

	urlValue, err := normalizeSystemConfigValue("AI_API_BASE", "https://api.example.com/v1")
	if err != nil {
		t.Fatalf("normalize url failed: %v", err)
	}
	if urlValue != "https://api.example.com/v1" {
		t.Fatalf("unexpected url value: %q", urlValue)
	}
	if _, err := normalizeSystemConfigValue("AI_API_BASE", "localhost:8080"); err == nil {
		t.Fatalf("expected invalid URL to fail")
	}
}

func TestNormalizeSystemConfigValueDefaultPassthrough(t *testing.T) {
	got, err := normalizeSystemConfigValue("UNKNOWN_KEY", "  value  ")
	if err != nil {
		t.Fatalf("normalize default failed: %v", err)
	}
	if got != "value" {
		t.Fatalf("unexpected passthrough value: %q", got)
	}
}

func TestIsBackupSchedulerRelatedKey(t *testing.T) {
	if !isBackupSchedulerRelatedKey("BACKUP_ENABLED") {
		t.Fatalf("BACKUP_ENABLED should require scheduler reload")
	}
	if !isBackupSchedulerRelatedKey("BACKUP_SCHEDULE") {
		t.Fatalf("BACKUP_SCHEDULE should require scheduler reload")
	}
	if isBackupSchedulerRelatedKey("MAIL_PORT") {
		t.Fatalf("MAIL_PORT should not require scheduler reload")
	}
}

func TestIsWeeklyReportSchedulerRelatedKey(t *testing.T) {
	if !isWeeklyReportSchedulerRelatedKey("WEEKLY_REPORT_ENABLED") {
		t.Fatalf("WEEKLY_REPORT_ENABLED should require weekly scheduler reload")
	}
	if !isWeeklyReportSchedulerRelatedKey("WEEKLY_REPORT_SCHEDULE") {
		t.Fatalf("WEEKLY_REPORT_SCHEDULE should require weekly scheduler reload")
	}
	if isWeeklyReportSchedulerRelatedKey("MAIL_PORT") {
		t.Fatalf("MAIL_PORT should not require weekly scheduler reload")
	}
}
