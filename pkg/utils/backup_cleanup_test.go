package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupOldBackupFiles(t *testing.T) {
	dir := t.TempDir()

	oldEnc := filepath.Join(dir, "old-backup.enc")
	newEnc := filepath.Join(dir, "new-backup.enc")
	oldTxt := filepath.Join(dir, "old-note.txt")

	if err := os.WriteFile(oldEnc, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old enc failed: %v", err)
	}
	if err := os.WriteFile(newEnc, []byte("new"), 0o600); err != nil {
		t.Fatalf("write new enc failed: %v", err)
	}
	if err := os.WriteFile(oldTxt, []byte("note"), 0o600); err != nil {
		t.Fatalf("write txt failed: %v", err)
	}

	now := time.Now()
	veryOld := now.AddDate(0, 0, -40)
	recent := now.AddDate(0, 0, -2)

	if err := os.Chtimes(oldEnc, veryOld, veryOld); err != nil {
		t.Fatalf("chtimes old enc failed: %v", err)
	}
	if err := os.Chtimes(newEnc, recent, recent); err != nil {
		t.Fatalf("chtimes new enc failed: %v", err)
	}
	if err := os.Chtimes(oldTxt, veryOld, veryOld); err != nil {
		t.Fatalf("chtimes txt failed: %v", err)
	}

	removed, err := CleanupOldBackupFiles(dir, 30)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed file, got %d (%v)", len(removed), removed)
	}
	if removed[0] != oldEnc {
		t.Fatalf("expected removed file %s, got %s", oldEnc, removed[0])
	}

	if _, err := os.Stat(oldEnc); !os.IsNotExist(err) {
		t.Fatalf("expected old enc removed")
	}
	if _, err := os.Stat(newEnc); err != nil {
		t.Fatalf("expected new enc kept, stat err: %v", err)
	}
	if _, err := os.Stat(oldTxt); err != nil {
		t.Fatalf("expected non-enc file kept, stat err: %v", err)
	}
}

func TestCleanupOldBackupFilesInvalidInput(t *testing.T) {
	removed, err := CleanupOldBackupFiles("", 30)
	if err != nil {
		t.Fatalf("unexpected error for empty dir: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removed files for empty dir")
	}

	dir := t.TempDir()
	removed, err = CleanupOldBackupFiles(dir, 0)
	if err != nil {
		t.Fatalf("unexpected error for invalid retention: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removed files for invalid retention")
	}
}
