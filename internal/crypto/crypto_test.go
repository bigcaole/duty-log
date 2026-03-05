package crypto

import "testing"

func TestDeriveAES256Key(t *testing.T) {
	key := DeriveAES256Key("unit-test-secret")
	if len(key) != 32 {
		t.Fatalf("expected key length 32, got %d", len(key))
	}

	key2 := DeriveAES256Key("unit-test-secret")
	if string(key) != string(key2) {
		t.Fatalf("expected deterministic key derivation")
	}
}

func TestEncryptDecryptAES256GCM(t *testing.T) {
	key := DeriveAES256Key("another-secret")
	plaintext := "hello duty-log"

	encoded, err := EncryptAES256GCM(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if encoded == "" {
		t.Fatalf("expected non-empty ciphertext")
	}

	decoded, err := DecryptAES256GCM(key, encoded)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decoded != plaintext {
		t.Fatalf("expected plaintext %q, got %q", plaintext, decoded)
	}
}

func TestDecryptAES256GCMWrongKey(t *testing.T) {
	key := DeriveAES256Key("key-a")
	wrongKey := DeriveAES256Key("key-b")

	encoded, err := EncryptAES256GCM(key, "secret")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if _, err := DecryptAES256GCM(wrongKey, encoded); err == nil {
		t.Fatalf("expected decrypt with wrong key to fail")
	}
}
