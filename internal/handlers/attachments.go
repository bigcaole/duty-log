package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type attachmentViewItem struct {
	ID          uint
	Name     string
	URL      string
	SizeText string
	IsImage  bool
	DeleteValue string
}

var fileNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

const (
	defaultUploadDir     = "./static/uploads"
	defaultUploadURLBase = "/static/uploads"
)

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

	baseDir := normalizeUploadDir(strings.TrimSpace(os.Getenv("UPLOAD_DIR")))
	baseURL := normalizeUploadURLBase(strings.TrimSpace(os.Getenv("UPLOAD_URL_BASE")))

	dateDir := time.Now().Format("20060102")
	relDir := filepath.Join(moduleDir, dateDir)
	absDir := filepath.Join(baseDir, relDir)
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

		urlPath := baseURL + "/" + filepath.ToSlash(filepath.Join(relDir, storedName))
		attachments = append(attachments, map[string]any{
			"name": file.Filename,
			"url":  urlPath,
			"size": file.Size,
			"path": filepath.ToSlash(filepath.Join(relDir, storedName)),
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
		if url == "" {
			if path := strings.TrimSpace(fmt.Sprintf("%v", row["path"])); path != "" {
				url = normalizeUploadURLBase(strings.TrimSpace(os.Getenv("UPLOAD_URL_BASE"))) + "/" + strings.TrimPrefix(path, "/")
			}
		}
		sizeText := humanReadableSize(row["size"])
		if name == "" {
			name = "附件"
		}
		if url == "" {
			continue
		}
		isImage := isImageAttachment(name, url)
		items = append(items, attachmentViewItem{
			Name:     name,
			URL:      url,
			SizeText: sizeText,
			IsImage:  isImage,
			DeleteValue: "fs:" + url,
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

func isImageAttachment(name, url string) bool {
	candidates := []string{name, url}
	for _, candidate := range candidates {
		ext := strings.ToLower(filepath.Ext(strings.TrimSpace(candidate)))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
			return true
		}
	}
	return false
}

type uploadFile struct {
	Name        string
	ContentType string
	Size        int64
	Data        []byte
}

func readUploadedFiles(c *gin.Context, formField string) ([]uploadFile, error) {
	if c == nil {
		return nil, nil
	}
	form, err := c.MultipartForm()
	if err != nil {
		if err == http.ErrNotMultipart {
			return nil, nil
		}
		return nil, err
	}
	if form == nil || form.File == nil {
		return nil, nil
	}
	files := form.File[formField]
	if len(files) == 0 {
		return nil, nil
	}

	uploads := make([]uploadFile, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			return nil, err
		}
		contentType := strings.TrimSpace(file.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		uploads = append(uploads, uploadFile{
			Name:        file.Filename,
			ContentType: contentType,
			Size:        file.Size,
			Data:        data,
		})
	}
	return uploads, nil
}

func dbAttachmentViewItems(rows []models.Attachment) []attachmentViewItem {
	items := make([]attachmentViewItem, 0, len(rows))
	for _, row := range rows {
		url := fmt.Sprintf("/attachments/%d", row.ID)
		isImage := strings.HasPrefix(strings.ToLower(strings.TrimSpace(row.ContentType)), "image/")
		if !isImage {
			isImage = isImageAttachment(row.Name, url)
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = "附件"
		}
		items = append(items, attachmentViewItem{
			ID:          row.ID,
			Name:        name,
			URL:         url,
			SizeText:    humanReadableSize(row.Size),
			IsImage:     isImage,
			DeleteValue: fmt.Sprintf("db:%d", row.ID),
		})
	}
	return items
}

func loadAttachmentViewItems(db *gorm.DB, module string, moduleID uint, fsRows models.JSONSlice) []attachmentViewItem {
	items := parseAttachmentViewItems(fsRows)
	if db == nil || module == "" || moduleID == 0 {
		return items
	}
	var rows []models.Attachment
	if err := db.Where("module = ? AND module_id = ?", module, moduleID).Order("id asc").Find(&rows).Error; err != nil {
		return items
	}
	items = append(items, dbAttachmentViewItems(rows)...)
	return items
}

func parseAttachmentRemovals(values []string) (dbIDs []uint, fsURLs []string) {
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if strings.HasPrefix(value, "db:") {
			idStr := strings.TrimPrefix(value, "db:")
			if parsed, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64); err == nil && parsed > 0 {
				dbIDs = append(dbIDs, uint(parsed))
			}
			continue
		}
		if strings.HasPrefix(value, "fs:") {
			url := strings.TrimSpace(strings.TrimPrefix(value, "fs:"))
			if url != "" {
				fsURLs = append(fsURLs, url)
			}
		}
	}
	return dbIDs, fsURLs
}

func filterAttachmentRowsByURL(rows models.JSONSlice, removeURLs []string) models.JSONSlice {
	if len(removeURLs) == 0 {
		return rows
	}
	removeSet := make(map[string]struct{}, len(removeURLs))
	for _, url := range removeURLs {
		cleaned := strings.TrimSpace(url)
		if cleaned == "" {
			continue
		}
		removeSet[cleaned] = struct{}{}
	}
	if len(removeSet) == 0 {
		return rows
	}
	filtered := make(models.JSONSlice, 0, len(rows))
	for _, row := range rows {
		url := strings.TrimSpace(fmt.Sprintf("%v", row["url"]))
		if url != "" {
			if _, ok := removeSet[url]; ok {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	if len(filtered) == 0 {
		return models.JSONSlice{}
	}
	return filtered
}

func saveAttachmentsToDB(tx *gorm.DB, module string, moduleID uint, uploads []uploadFile) error {
	if tx == nil || module == "" || moduleID == 0 || len(uploads) == 0 {
		return nil
	}
	for _, up := range uploads {
		name := strings.TrimSpace(up.Name)
		if name == "" {
			name = "附件"
		}
		contentType := strings.TrimSpace(up.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(up.Data)
		}
		size := up.Size
		if size <= 0 {
			size = int64(len(up.Data))
		}
		attachment := models.Attachment{
			Module:      module,
			ModuleID:    moduleID,
			Name:        name,
			ContentType: contentType,
			Size:        size,
			Data:        up.Data,
		}
		if err := tx.Create(&attachment).Error; err != nil {
			return err
		}
	}
	return nil
}

func attachmentCountByModule(db *gorm.DB, module string, ids []uint) map[uint]int {
	result := make(map[uint]int)
	if db == nil || module == "" || len(ids) == 0 {
		return result
	}
	type countRow struct {
		ModuleID uint
		Count    int
	}
	var rows []countRow
	if err := db.Model(&models.Attachment{}).
		Select("module_id, count(*) as count").
		Where("module = ? AND module_id IN ?", module, ids).
		Group("module_id").
		Find(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ModuleID] = row.Count
	}
	return result
}
func UploadFileHandler(baseDir string) gin.HandlerFunc {
	resolved := normalizeUploadDir(strings.TrimSpace(baseDir))
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" || strings.Contains(rel, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		serveFile(c, resolved, rel, true)
	}
}

func StaticFileHandler(uploadDir, staticDir string) gin.HandlerFunc {
	resolvedUpload := normalizeUploadDir(strings.TrimSpace(uploadDir))
	resolvedStatic := strings.TrimSpace(staticDir)
	if resolvedStatic == "" {
		resolvedStatic = "./static"
	}
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" || strings.Contains(rel, "..") {
			c.Status(http.StatusNotFound)
			return
		}

		if strings.HasPrefix(rel, "uploads"+string(filepath.Separator)) || strings.HasPrefix(rel, "uploads/") {
			relUpload := strings.TrimPrefix(rel, "uploads/")
			relUpload = strings.TrimPrefix(relUpload, "uploads"+string(filepath.Separator))
			serveFile(c, resolvedUpload, relUpload, true)
			return
		}

		serveFile(c, resolvedStatic, rel, false)
	}
}

func serveFile(c *gin.Context, baseDir, rel string, inline bool) {
	absPath := filepath.Join(baseDir, rel)
	file, err := os.Open(absPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}

	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, 0); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", contentType)
	if inline {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(info.Name())))
	}
	c.DataFromReader(http.StatusOK, info.Size(), contentType, file, nil)
}

func normalizeUploadDir(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return defaultUploadDir
	}
	return trimmed
}

func normalizeUploadURLBase(url string) string {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return defaultUploadURLBase
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return defaultUploadURLBase
	}
	return trimmed
}
