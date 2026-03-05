package handlers

import "testing"

func TestSystemConfigDefinitionsUniqueKeys(t *testing.T) {
	seen := make(map[string]struct{}, len(systemConfigDefinitions))
	for _, def := range systemConfigDefinitions {
		if def.Key == "" {
			t.Fatalf("definition key should not be empty")
		}
		if _, exists := seen[def.Key]; exists {
			t.Fatalf("duplicate config key found: %s", def.Key)
		}
		seen[def.Key] = struct{}{}
	}
}

func TestSystemConfigKeysMatchesDefinitions(t *testing.T) {
	keys := systemConfigKeys()
	if len(keys) != len(systemConfigDefinitions) {
		t.Fatalf("keys length mismatch: got %d want %d", len(keys), len(systemConfigDefinitions))
	}
	for i, key := range keys {
		if key != systemConfigDefinitions[i].Key {
			t.Fatalf("key index %d mismatch: got %s want %s", i, key, systemConfigDefinitions[i].Key)
		}
	}
}

func TestIsSensitiveSystemConfigKey(t *testing.T) {
	if !isSensitiveSystemConfigKey("AI_API_KEY") {
		t.Fatalf("AI_API_KEY should be sensitive")
	}
	if !isSensitiveSystemConfigKey("MAIL_PASSWORD") {
		t.Fatalf("MAIL_PASSWORD should be sensitive")
	}
	if isSensitiveSystemConfigKey("BACKUP_ENABLED") {
		t.Fatalf("BACKUP_ENABLED should not be sensitive")
	}
}

func TestShouldSkipSensitiveBulkUpdate(t *testing.T) {
	if !shouldSkipSensitiveBulkUpdate("AI_API_KEY", "") {
		t.Fatalf("empty sensitive value should be skipped")
	}
	if !shouldSkipSensitiveBulkUpdate("MAIL_PASSWORD", "   ") {
		t.Fatalf("blank sensitive value should be skipped")
	}
	if shouldSkipSensitiveBulkUpdate("AI_API_KEY", "new-value") {
		t.Fatalf("non-empty sensitive value should not be skipped")
	}
	if shouldSkipSensitiveBulkUpdate("BACKUP_ENABLED", "") {
		t.Fatalf("non-sensitive value should not be skipped")
	}
}

func TestMaskSystemConfigValue(t *testing.T) {
	if got := maskSystemConfigValue(""); got != "" {
		t.Fatalf("empty value mask mismatch: %q", got)
	}
	if got := maskSystemConfigValue("abcd"); got != "****" {
		t.Fatalf("short mask mismatch: %q", got)
	}
	if got := maskSystemConfigValue("abcdefghij"); got != "ab****ij" {
		t.Fatalf("long mask mismatch: %q", got)
	}
}

func TestIsLoginRateLimitRelatedKey(t *testing.T) {
	if !isLoginRateLimitRelatedKey("LOGIN_MAX_ATTEMPTS") {
		t.Fatalf("LOGIN_MAX_ATTEMPTS should be login-rate related")
	}
	if !isLoginRateLimitRelatedKey("LOGIN_WINDOW_SECONDS") {
		t.Fatalf("LOGIN_WINDOW_SECONDS should be login-rate related")
	}
	if !isLoginRateLimitRelatedKey("LOGIN_BLOCK_SECONDS") {
		t.Fatalf("LOGIN_BLOCK_SECONDS should be login-rate related")
	}
	if isLoginRateLimitRelatedKey("BACKUP_ENABLED") {
		t.Fatalf("BACKUP_ENABLED should not be login-rate related")
	}
}

func TestHasUpdatedSystemConfigKey(t *testing.T) {
	updates := []systemConfigUpdate{
		{Key: "MAIL_PORT", Value: "587"},
		{Key: "LOGIN_MAX_ATTEMPTS", Value: "10"},
	}
	if !hasUpdatedSystemConfigKey(updates, isLoginRateLimitRelatedKey) {
		t.Fatalf("expected login-rate key to be detected")
	}
	if hasUpdatedSystemConfigKey(updates, func(key string) bool { return key == "NOPE" }) {
		t.Fatalf("unexpected key detection")
	}
}
