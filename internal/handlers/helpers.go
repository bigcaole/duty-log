package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	dateLayout          = "2006-01-02"
	dateTimeLocalLayout = "2006-01-02T15:04"
)

func parseRequiredDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("日期不能为空")
	}
	parsed, err := time.ParseInLocation(dateLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("日期格式错误，应为 YYYY-MM-DD")
	}
	return parsed, nil
}

func parseRequiredDateTime(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("时间不能为空")
	}
	parsed, err := time.ParseInLocation(dateTimeLocalLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("时间格式错误")
	}
	return parsed, nil
}

func parseOptionalDateTime(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(dateTimeLocalLayout, value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("完成时间格式错误")
	}
	return &parsed, nil
}

func parseOptionalDate(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(dateLayout, value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误，应为 YYYY-MM-DD")
	}
	return &parsed, nil
}

func parseRequiredUint(raw, fieldName string) (uint, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s不能为空", fieldName)
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s格式错误", fieldName)
	}
	if n == 0 {
		return 0, fmt.Errorf("%s必须大于0", fieldName)
	}
	return uint(n), nil
}

func parseOptionalUint(raw string) (*uint, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("数字格式错误")
	}
	if n == 0 {
		return nil, nil
	}
	converted := uint(n)
	return &converted, nil
}

func parseBoolForm(c *gin.Context, key string) bool {
	value := strings.TrimSpace(strings.ToLower(c.PostForm(key)))
	switch value {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func todayDateString() string {
	return time.Now().Format(dateLayout)
}

func nowDateTimeLocalString() string {
	return time.Now().Format(dateTimeLocalLayout)
}

func canAccessOwnedRecord(isAdmin bool, ownerID uint, currentUserID uint) bool {
	if isAdmin {
		return true
	}
	return ownerID == currentUserID
}
