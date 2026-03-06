package handlers

import "testing"

func TestRequiredSetupConfigDefinitions(t *testing.T) {
	required := requiredSetupConfigDefinitions()
	if len(required) == 0 {
		t.Fatalf("required setup config definitions should not be empty")
	}

	expected := map[string]struct{}{
		"BACKUP_ENABLED":        {},
		"BACKUP_RETENTION_DAYS": {},
		"AUDIT_RETENTION_DAYS":  {},
		"LOGIN_MAX_ATTEMPTS":    {},
		"LOGIN_WINDOW_SECONDS":  {},
		"LOGIN_BLOCK_SECONDS":   {},
		"TOTP_ISSUER":           {},
	}

	actual := make(map[string]struct{}, len(required))
	for _, def := range required {
		actual[def.Key] = struct{}{}
	}
	for key := range expected {
		if _, ok := actual[key]; !ok {
			t.Fatalf("required setup key missing: %s", key)
		}
	}
}

func TestIsInitialSetupExemptPath(t *testing.T) {
	if !isInitialSetupExemptPath("/admin/setup/config") {
		t.Fatalf("setup config page should be exempt")
	}
	if !isInitialSetupExemptPath("/admin/setup/config?x=1") {
		t.Fatalf("setup config page with query should be exempt")
	}
	if isInitialSetupExemptPath("/admin/config") {
		t.Fatalf("admin config page should not be exempt")
	}
	if isInitialSetupExemptPath("/dashboard") {
		t.Fatalf("dashboard should not be exempt")
	}
}
