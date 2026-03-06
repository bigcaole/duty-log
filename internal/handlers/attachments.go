package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

type attachmentViewItem struct {
	Name     string
	URL      string
	SizeText string
}

var fileNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func saveUploadedAttachments(c *gin.Context, formField, moduleDir string) (models.JSONSlice, error) {
	if c == nil {
		return models.JSONSlice{}, nil
	}

	form, err := c.MultipartForm()
	if err != nil {
		if err == http.ErrNotMultipart {
			return models.JSONSlice{}, nil
		}
		return nil, err
	}
	if form == nil || form.File == nil {
		return models.JSONSlice{}, nil
	}

	files := form.File[formField]
	if len(files) == 0 {
		return models.JSONSlice{}, nil
	}

	dateDir := time.Now().Format("20060102")
	relDir := filepath.Join("uploads", moduleDir, dateDir)
	absDir := filepath.Join("static", relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, err
	}

	attachments := make(models.JSONSlice, 0, len(files))
	for idx, file := range files {
		if file == nil {
			continue
		}
		baseName := sanitizeUploadedFileName(file.Filename)
		if baseName == "" {
			baseName = "attachment.bin"
		}
		storedName := fmt.Sprintf("%d_%d_%s", time.Now().UnixNano(), idx, baseName)
		storedPath := filepath.Join(absDir, storedName)
		if err := c.SaveUploadedFile(file, storedPath); err != nil {
			return nil, err
		}

		urlPath := "/static/" + filepath.ToSlash(filepath.Join(relDir, storedName))
		attachments = append(attachments, map[string]any{
			"name": file.Filename,
			"url":  urlPath,
			"size": file.Size,
		})
	}

	return attachments, nil
}

func mergeAttachments(existing, incoming models.JSONSlice) models.JSONSlice {
	result := make(models.JSONSlice, 0, len(existing)+len(incoming))
	for _, item := range existing {
		result = append(result, item)
	}
	for _, item := range incoming {
		result = append(result, item)
	}
	if len(result) == 0 {
		return models.JSONSlice{}
	}
	return result
}

func parseAttachmentViewItems(rows models.JSONSlice) []attachmentViewItem {
	items := make([]attachmentViewItem, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
		url := strings.TrimSpace(fmt.Sprintf("%v", row["url"]))
		sizeText := humanReadableSize(row["size"])
		if name == "" {
			name = "附件"
		}
		if url == "" {
			continue
		}
		items = append(items, attachmentViewItem{
			Name:     name,
			URL:      url,
			SizeText: sizeText,
		})
	}
	return items
}

func sanitizeUploadedFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, " ", "_")
	base = fileNameSanitizer.ReplaceAllString(base, "_")
	return strings.Trim(base, "._")
}

func humanReadableSize(raw any) string {
	var size int64
	switch v := raw.(type) {
	case int:
		size = int64(v)
	case int32:
		size = int64(v)
	case int64:
		size = v
	case float64:
		size = int64(v)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			size = parsed
		}
	default:
		size = 0
	}

	if size <= 0 {
		return "-"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case size >= gb:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
