package utils

import (
	"strings"
	"testing"

	"duty-log-system/internal/crypto"
)

func TestResolveSensitiveConfigValueEncrypted(t *testing.T) {
	key := crypto.DeriveAES256Key("secret-a")
	encrypted, err := crypto.EncryptAES256GCM(key, "plain-value")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	plain, normalized, ok := resolveSensitiveConfigValue(key, wrapSensitiveCipher(encrypted))
	if !ok {
		t.Fatalf("expected encrypted value to resolve")
	}
	if plain != "plain-value" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
	if normalized != wrapSensitiveCipher(encrypted) {
		t.Fatalf("unexpected normalized value")
	}
}

func TestResolveSensitiveConfigValueLegacyPlaintext(t *testing.T) {
	key := crypto.DeriveAES256Key("secret-b")
	plain, normalized, ok := resolveSensitiveConfigValue(key, "legacy-plain")
	if !ok {
		t.Fatalf("expected legacy plaintext to resolve")
	}
	if plain != "legacy-plain" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
	if normalized == "" || normalized == "legacy-plain" || !strings.HasPrefix(normalized, sensitiveConfigCipherPrefix) {
		t.Fatalf("expected normalized encrypted value, got %q", normalized)
	}

	decoded, err := crypto.DecryptAES256GCM(key, strings.TrimPrefix(normalized, sensitiveConfigCipherPrefix))
	if err != nil {
		t.Fatalf("decrypt normalized failed: %v", err)
	}
	if decoded != "legacy-plain" {
		t.Fatalf("unexpected decrypted normalized value: %q", decoded)
	}
}

func TestResolveSensitiveConfigValueWrongKeyCiphertext(t *testing.T) {
	keyA := crypto.DeriveAES256Key("secret-a")
	keyB := crypto.DeriveAES256Key("secret-b")
	encrypted, err := crypto.EncryptAES256GCM(keyA, "value-a")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	plain, normalized, ok := resolveSensitiveConfigValue(keyB, wrapSensitiveCipher(encrypted))
	if ok {
		t.Fatalf("expected wrong-key ciphertext to be unresolved, got plain=%q normalized=%q", plain, normalized)
	}
}

func TestLooksLikeEncryptedConfigValue(t *testing.T) {
	key := crypto.DeriveAES256Key("secret-c")
	encrypted, err := crypto.EncryptAES256GCM(key, "value-c")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if !looksLikeEncryptedConfigValue(encrypted) {
		t.Fatalf("encrypted value should look encrypted")
	}
	if looksLikeEncryptedConfigValue("not-base64") {
		t.Fatalf("plain text should not look encrypted")
	}
	if looksLikeEncryptedConfigValue("") {
		t.Fatalf("empty string should not look encrypted")
	}
}

func TestResolveSensitiveConfigValueLegacyEncryptedWithoutPrefix(t *testing.T) {
	key := crypto.DeriveAES256Key("secret-d")
	encrypted, err := crypto.EncryptAES256GCM(key, "value-d")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	plain, normalized, ok := resolveSensitiveConfigValue(key, encrypted)
	if !ok {
		t.Fatalf("expected legacy encrypted value to resolve")
	}
	if plain != "value-d" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
	if normalized != wrapSensitiveCipher(encrypted) {
		t.Fatalf("expected normalized prefixed ciphertext, got %q", normalized)
	}
}

func TestWrapAndUnwrapSensitiveCipher(t *testing.T) {
	wrapped := wrapSensitiveCipher("abc123")
	if wrapped != sensitiveConfigCipherPrefix+"abc123" {
		t.Fatalf("unexpected wrapped value: %q", wrapped)
	}
	raw, prefixed := unwrapSensitiveCipher(wrapped)
	if !prefixed || raw != "abc123" {
		t.Fatalf("unexpected unwrap result: raw=%q prefixed=%v", raw, prefixed)
	}
	raw, prefixed = unwrapSensitiveCipher("abc123")
	if prefixed || raw != "abc123" {
		t.Fatalf("unexpected unwrap plain result: raw=%q prefixed=%v", raw, prefixed)
	}
}
