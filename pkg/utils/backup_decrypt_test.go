package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptAndDecryptBackupFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "sample.zip")
	encPath := filepath.Join(tmpDir, "sample.zip.enc")

	original := []byte("this-is-a-fake-zip-content-for-test")
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}

	password := "UnitTestPassword123"
	if err := encryptFileToBase64(sourcePath, encPath, password); err != nil {
		t.Fatalf("encrypt file failed: %v", err)
	}

	decryptedPath, err := DecryptBackupBase64FileToTemp(encPath, password)
	if err != nil {
		t.Fatalf("decrypt file failed: %v", err)
	}
	defer os.Remove(decryptedPath)

	got, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("read decrypted file failed: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("decrypted content mismatch, got %q want %q", string(got), string(original))
	}
}

func TestDecryptBackupFileWrongPassword(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "sample.zip")
	encPath := filepath.Join(tmpDir, "sample.zip.enc")

	if err := os.WriteFile(sourcePath, []byte("fake-content"), 0o600); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}

	if err := encryptFileToBase64(sourcePath, encPath, "correct-password"); err != nil {
		t.Fatalf("encrypt file failed: %v", err)
	}

	if _, err := DecryptBackupBase64FileToTemp(encPath, "wrong-password"); err == nil {
		t.Fatalf("expected decrypt with wrong password to fail")
	}
}

func TestDecryptBackupFileCreatesUniqueTempFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "sample.zip")
	encPath := filepath.Join(tmpDir, "sample.zip.enc")

	if err := os.WriteFile(sourcePath, []byte("unique-test-content"), 0o600); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := encryptFileToBase64(sourcePath, encPath, "same-password"); err != nil {
		t.Fatalf("encrypt file failed: %v", err)
	}

	path1, err := DecryptBackupBase64FileToTemp(encPath, "same-password")
	if err != nil {
		t.Fatalf("first decrypt failed: %v", err)
	}
	defer os.Remove(path1)

	path2, err := DecryptBackupBase64FileToTemp(encPath, "same-password")
	if err != nil {
		t.Fatalf("second decrypt failed: %v", err)
	}
	defer os.Remove(path2)

	if path1 == path2 {
		t.Fatalf("expected unique temp files, got same path %q", path1)
	}
	if filepath.Ext(path1) != ".zip" || filepath.Ext(path2) != ".zip" {
		t.Fatalf("expected .zip temp files, got %q and %q", path1, path2)
	}
}
