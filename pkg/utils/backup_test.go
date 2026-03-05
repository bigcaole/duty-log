package utils

import (
	"strings"
	"testing"
)

func TestGenerateRandomPasswordLengthAndCharset(t *testing.T) {
	password, err := generateRandomPassword(24)
	if err != nil {
		t.Fatalf("generate password failed: %v", err)
	}
	if len(password) != 24 {
		t.Fatalf("expected length 24, got %d", len(password))
	}

	const allowed = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"
	for i := 0; i < len(password); i++ {
		if !strings.ContainsRune(allowed, rune(password[i])) {
			t.Fatalf("password contains invalid char %q", password[i])
		}
	}
}

func TestGenerateRandomPasswordDefaultLength(t *testing.T) {
	password, err := generateRandomPassword(0)
	if err != nil {
		t.Fatalf("generate password failed: %v", err)
	}
	if len(password) != 16 {
		t.Fatalf("expected default length 16, got %d", len(password))
	}
}
