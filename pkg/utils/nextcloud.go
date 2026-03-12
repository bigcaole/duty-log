package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type NextcloudConfig struct {
	BaseURL    string
	Username   string
	Password   string
	RemotePath string
	Timeout    time.Duration
}

func UploadToNextcloud(ctx context.Context, cfg NextcloudConfig, localFilePath string) error {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	user := strings.TrimSpace(cfg.Username)
	pass := strings.TrimSpace(cfg.Password)
	remotePath := strings.TrimSpace(cfg.RemotePath)
	if baseURL == "" || user == "" || pass == "" {
		return errors.New("nextcloud config incomplete")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	info, err := os.Stat(localFilePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("nextcloud upload expects file, got directory")
	}

	fileName := filepath.Base(localFilePath)
	remotePath = strings.Trim(remotePath, "/")
	fullPath := path.Join("remote.php", "dav", "files", user)
	if remotePath != "" {
		fullPath = path.Join(fullPath, remotePath)
	}
	fullPath = path.Join(fullPath, fileName)

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	parsed.Path = path.Join(parsed.Path, fullPath)

	file, err := os.Open(localFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, parsed.String(), file)
	if err != nil {
		return err
	}
	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = info.Size()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("nextcloud upload failed: status %d", resp.StatusCode)
	}
	return nil
}
