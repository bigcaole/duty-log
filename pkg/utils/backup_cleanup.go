package utils

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CleanupOldBackupFiles(dir string, retentionDays int) ([]string, error) {
	if strings.TrimSpace(dir) == "" || retentionDays <= 0 {
		return nil, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	removed := make([]string, 0, 16)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".enc") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}

		if err := os.Remove(path); err != nil {
			return nil
		}
		removed = append(removed, path)
		return nil
	})
	return removed, err
}
