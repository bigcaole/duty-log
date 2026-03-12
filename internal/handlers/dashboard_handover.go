package handlers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"duty-log-system/internal/models"
)

type dashboardHandoverOperationItem struct {
	Time     string
	Username string
	Action   string
	Module   string
	Path     string
}

type dashboardHandoverRecordItem struct {
	Time     string
	Username string
	Module   string
	Title    string
	Content  string
}

type dashboardReminderAlertItem struct {
	ID            uint
	Title         string
	Content       string
	StartDate     string
	EndDate       string
	Creator       string
	RemainingText string
	SeverityClass string
}

func (a *AppContext) loadYesterdayHandover(now time.Time) ([]dashboardHandoverOperationItem, []dashboardHandoverRecordItem, error) {
	dayStart := normalizeToDate(now).AddDate(0, 0, -1)
	dayEnd := dayStart.Add(24 * time.Hour)

	operations, err := a.loadYesterdayOperations(dayStart, dayEnd)
	if err != nil {
		return nil, nil, err
	}
	records, recordErr := a.loadYesterdayBusinessRecords(dayStart, dayEnd)
	if recordErr != nil {
		return operations, nil, recordErr
	}
	return operations, records, nil
}

func (a *AppContext) loadYesterdayOperations(dayStart, dayEnd time.Time) ([]dashboardHandoverOperationItem, error) {
	var rows []models.AuditLog
	if err := a.DB.Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Order("created_at desc").
		Limit(200).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	usernameByID := a.lookupUsernamesFromPtr(rows)
	items := make([]dashboardHandoverOperationItem, 0, len(rows))
	for _, row := range rows {
		username := usernameFromPtr(usernameByID, row.UserID)
		path := "-"
		if rawPath, ok := row.DetailsJSON["path"]; ok {
			path = strings.TrimSpace(fmt.Sprintf("%v", rawPath))
		}
		items = append(items, dashboardHandoverOperationItem{
			Time:     row.CreatedAt.Format("15:04:05"),
			Username: username,
			Action:   strings.TrimSpace(row.Action),
			Module:   tableNameLabel(row.TableName),
			Path:     path,
		})
	}
	return items, nil
}

func (a *AppContext) loadYesterdayBusinessRecords(dayStart, dayEnd time.Time) ([]dashboardHandoverRecordItem, error) {
	type timelineRecord struct {
		When    time.Time
		UserID  *uint
		Module  string
		Title   string
		Content string
	}

	timeline := make([]timelineRecord, 0, 256)

	var dutyLogs []models.DutyLog
	if err := a.DB.Where("updated_at >= ? AND updated_at < ?", dayStart, dayEnd).
		Order("updated_at desc").
		Limit(100).
		Find(&dutyLogs).Error; err != nil {
		return nil, err
	}
	for _, row := range dutyLogs {
		timeline = append(timeline, timelineRecord{
			When:    row.UpdatedAt,
			UserID:  row.UserID,
			Module:  "值班日志",
			Title:   fmt.Sprintf("%s 值班日志", row.Date.Format(dateLayout)),
			Content: trimDashboardText(row.Content, 160),
		})
	}

	var idcRecords []models.IdcDutyRecord
	if err := a.DB.Where("updated_at >= ? AND updated_at < ?", dayStart, dayEnd).
		Order("updated_at desc").
		Limit(100).
		Find(&idcRecords).Error; err != nil {
		return nil, err
	}
	for _, row := range idcRecords {
		content := strings.TrimSpace(row.Tasks)
		if content == "" {
			content = fmt.Sprintf("运维值班: %s, 机房值班: %s", strings.TrimSpace(row.DutyOps), strings.TrimSpace(row.DutyIdc))
		}
		timeline = append(timeline, timelineRecord{
			When:    row.UpdatedAt,
			UserID:  row.UserID,
			Module:  "IDC 值班",
			Title:   fmt.Sprintf("%s IDC 值班", row.Date.Format(dateLayout)),
			Content: trimDashboardText(content, 160),
		})
	}

	var idcOpsTickets []models.IDCOpsTicket
	if err := a.DB.Where("updated_at >= ? AND updated_at < ?", dayStart, dayEnd).
		Order("updated_at desc").
		Limit(100).
		Find(&idcOpsTickets).Error; err != nil {
		return nil, err
	}
	for _, row := range idcOpsTickets {
		userID := row.UserID
		timeline = append(timeline, timelineRecord{
			When:    row.UpdatedAt,
			UserID:  &userID,
			Module:  "IDC运维工单",
			Title:   trimDashboardText(row.VisitorOrganization, 80),
			Content: trimDashboardText(fmt.Sprintf("人数: %d | 事由: %s", row.VisitorCount, row.VisitorReason), 160),
		})
	}

	var workTickets []models.WorkTicket
	if err := a.DB.Where("updated_at >= ? AND updated_at < ?", dayStart, dayEnd).
		Order("updated_at desc").
		Limit(100).
		Find(&workTickets).Error; err != nil {
		return nil, err
	}
	for _, row := range workTickets {
		userID := row.UserID
		timeline = append(timeline, timelineRecord{
			When:    row.UpdatedAt,
			UserID:  &userID,
			Module:  "网络运维工单",
			Title:   trimDashboardText(row.UserName, 80),
			Content: trimDashboardText(fmt.Sprintf("状态: %s | 操作: %s", processingStatusLabel(row.ProcessingStatus), row.OperationInfo), 160),
		})
	}

	var faultRecords []models.FaultRecord
	if err := a.DB.Where("updated_at >= ? AND updated_at < ?", dayStart, dayEnd).
		Order("updated_at desc").
		Limit(100).
		Find(&faultRecords).Error; err != nil {
		return nil, err
	}
	for _, row := range faultRecords {
		userID := row.UserID
		timeline = append(timeline, timelineRecord{
			When:    row.UpdatedAt,
			UserID:  &userID,
			Module:  "网络故障记录",
			Title:   trimDashboardText(row.UserName, 80),
			Content: trimDashboardText(fmt.Sprintf("状态: %s | 故障: %s", processingStatusLabel(row.ProcessingStatus), row.FaultSymptom), 160),
		})
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].When.After(timeline[j].When)
	})
	if len(timeline) > 200 {
		timeline = timeline[:200]
	}

	userIDs := make([]uint, 0, len(timeline))
	userIDSet := make(map[uint]struct{}, len(timeline))
	for _, row := range timeline {
		if row.UserID == nil {
			continue
		}
		if _, ok := userIDSet[*row.UserID]; ok {
			continue
		}
		userIDSet[*row.UserID] = struct{}{}
		userIDs = append(userIDs, *row.UserID)
	}
	usernameByID := a.lookupUsernames(userIDs)
	items := make([]dashboardHandoverRecordItem, 0, len(timeline))
	for _, row := range timeline {
		items = append(items, dashboardHandoverRecordItem{
			Time:     row.When.Format("15:04:05"),
			Username: usernameFromPtr(usernameByID, row.UserID),
			Module:   row.Module,
			Title:    row.Title,
			Content:  row.Content,
		})
	}
	return items, nil
}

func (a *AppContext) loadHomepageReminderAlerts(now time.Time, currentUser *models.User) ([]dashboardReminderAlertItem, error) {
	var rows []models.Reminder
	query := a.DB.Where("is_completed = ?", false)
	if currentUser != nil && !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if err := query.
		Order("end_date asc, updated_at desc").
		Limit(300).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	userIDs := make([]uint, 0, len(rows))
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.UserID]; ok {
			continue
		}
		seen[row.UserID] = struct{}{}
		userIDs = append(userIDs, row.UserID)
	}

	var users []models.User
	if len(userIDs) > 0 {
		_ = a.DB.Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error
	}
	usernameByID := make(map[uint]string, len(users))
	for _, user := range users {
		usernameByID[user.ID] = user.Username
	}

	today := now
	items := make([]dashboardReminderAlertItem, 0, len(rows))
	for _, row := range rows {
		deadline := reminderDeadlineTime(row)
		triggerTime := reminderTriggerTime(row)
		if today.Before(triggerTime) {
			continue
		}

		remainingText := ""
		severityClass := "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200"
		if today.After(deadline) {
			overdueDays := int(today.Sub(deadline).Hours() / 24)
			if overdueDays < 1 {
				overdueDays = 1
			}
			remainingText = fmt.Sprintf("已逾期 %d 天", overdueDays)
			severityClass = "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200"
		} else {
			remainingDays := int(deadline.Sub(today).Hours() / 24)
			if remainingDays <= 0 {
				remainingText = "即将到期"
			} else {
				remainingText = fmt.Sprintf("%d 天后到期", remainingDays)
			}
		}

		creator := "-"
		if username, ok := usernameByID[row.UserID]; ok {
			creator = username
		}

		items = append(items, dashboardReminderAlertItem{
			ID:            row.ID,
			Title:         row.Title,
			Content:       trimDashboardText(row.Content, 150),
			StartDate:     row.StartDate.Format(dateLayout),
			EndDate:       row.EndDate.Format(dateLayout),
			Creator:       creator,
			RemainingText: remainingText,
			SeverityClass: severityClass,
		})
	}

	if len(items) > 20 {
		items = items[:20]
	}
	return items, nil
}

func (a *AppContext) lookupUsernamesFromPtr(logs []models.AuditLog) map[uint]string {
	ids := make([]uint, 0, len(logs))
	seen := make(map[uint]struct{}, len(logs))
	for _, row := range logs {
		if row.UserID == nil {
			continue
		}
		if _, ok := seen[*row.UserID]; ok {
			continue
		}
		seen[*row.UserID] = struct{}{}
		ids = append(ids, *row.UserID)
	}
	return a.lookupUsernames(ids)
}

func (a *AppContext) lookupUsernames(ids []uint) map[uint]string {
	if len(ids) == 0 {
		return map[uint]string{}
	}
	var users []models.User
	_ = a.DB.Select("id", "username").Where("id IN ?", ids).Find(&users).Error
	result := make(map[uint]string, len(users))
	for _, user := range users {
		result[user.ID] = user.Username
	}
	return result
}

func usernameFromPtr(usernameByID map[uint]string, userID *uint) string {
	if userID == nil {
		return "system"
	}
	if username, ok := usernameByID[*userID]; ok {
		return username
	}
	return fmt.Sprintf("user#%d", *userID)
}

func tableNameLabel(table string) string {
	switch strings.TrimSpace(table) {
	case "duty_logs":
		return "值班日志"
	case "idc_duty_records":
		return "IDC 值班"
	case "tickets":
		return "历史普通工单"
	case "idc_ops_tickets":
		return "IDC运维工单"
	case "work_tickets":
		return "网络运维工单"
	case "fault_records":
		return "网络故障记录"
	case "system_configs":
		return "系统配置"
	case "backup_notifications":
		return "备份"
	case "users":
		return "用户管理"
	default:
		if strings.TrimSpace(table) == "" {
			return "其他"
		}
		return table
	}
}

func trimDashboardText(value string, max int) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "-"
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}
