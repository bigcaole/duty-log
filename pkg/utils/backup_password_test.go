package utils

import "testing"

func TestEncryptDecryptBackupPassword(t *testing.T) {
	secret := "super-secret-key"
	plain := "Abc123!"

	stored, err := EncryptBackupPassword(secret, plain)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if stored == "" {
		t.Fatalf("expected non-empty stored value")
	}
	if stored == plain {
		t.Fatalf("expected ciphertext not equal plaintext")
	}

	decoded, err := DecryptBackupPassword(secret, stored)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decoded != plain {
		t.Fatalf("unexpected plaintext, got %q want %q", decoded, plain)
	}
}

func TestDecryptBackupPasswordBackwardCompatibilityPlaintext(t *testing.T) {
	plain := "legacy-plain-password"
	decoded, err := DecryptBackupPassword("ignored-secret", plain)
	if err != nil {
		t.Fatalf("decrypt legacy plaintext should not fail: %v", err)
	}
	if decoded != plain {
		t.Fatalf("expected legacy plaintext unchanged")
	}
}

func TestDecryptBackupPasswordWrongSecret(t *testing.T) {
	stored, err := EncryptBackupPassword("secret-a", "password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if _, err := DecryptBackupPassword("secret-b", stored); err == nil {
		t.Fatalf("expected decrypt with wrong secret to fail")
	}
}

func TestResolveBackupPasswordEncrypted(t *testing.T) {
	stored, err := EncryptBackupPassword("secret-1", "value-1")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	plain, normalized, err := ResolveBackupPassword("secret-1", stored)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if plain != "value-1" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
	if normalized != stored {
		t.Fatalf("expected normalized to equal original encrypted value")
	}
}

func TestResolveBackupPasswordLegacyPlaintext(t *testing.T) {
	plain, normalized, err := ResolveBackupPassword("secret-2", "legacy-pass")
	if err != nil {
		t.Fatalf("resolve legacy plaintext failed: %v", err)
	}
	if plain != "legacy-pass" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
	if normalized == "" || normalized == "legacy-pass" {
		t.Fatalf("expected normalized encrypted value, got %q", normalized)
	}

	decoded, err := DecryptBackupPassword("secret-2", normalized)
	if err != nil {
		t.Fatalf("decrypt normalized failed: %v", err)
	}
	if decoded != "legacy-pass" {
		t.Fatalf("unexpected decoded value: %q", decoded)
	}
}
