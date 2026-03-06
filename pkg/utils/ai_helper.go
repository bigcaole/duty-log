package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"duty-log-system/internal/models"

	"gorm.io/gorm"
)

type DutyRecord struct {
	Date    time.Time
	Content string
}

type IDCOpsRecord struct {
	Date                time.Time
	VisitorOrganization string
	VisitorCount        int
	VisitorReason       string
	CustomerService     string
}

type WorkTicketRecord struct {
	Date           time.Time
	UserName       string
	TicketTypeName string
	OperationInfo  string
	Status         string
}

type NetworkFaultRecord struct {
	Date          time.Time
	UserName      string
	FaultSymptom  string
	Process       string
	Status        string
	FaultTypeName string
}

type WeeklySummaryResult struct {
	ReportType            string
	ReportTypeLabel       string
	PeriodStart           time.Time
	PeriodEnd             time.Time
	GeneratedAt           time.Time
	Summary               string
	DutyCount             int
	IDCOpsTicketCount     int
	WorkTicketCount       int
	NetworkFaultCount     int
	TicketCount           int
	LegacyWorkTicketCount int
}

type chatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []map[string]string `json:"messages"`
	Temperature float64             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func GenerateWeeklySummary(ctx context.Context, db *gorm.DB, configCenter *ConfigCenter, now time.Time) (WeeklySummaryResult, error) {
	return GeneratePeriodicSummary(ctx, db, configCenter, now, "week")
}

func GeneratePeriodicSummary(ctx context.Context, db *gorm.DB, configCenter *ConfigCenter, now time.Time, reportType string) (WeeklySummaryResult, error) {
	periodType := normalizeReportType(reportType)
	periodStart, periodEnd, periodLabel := reportWindow(now, periodType)

	dutyRecords, idcOpsRecords, workTicketRecords, faultRecords, err := fetchPeriodicRecords(db, periodStart, periodEnd)
	if err != nil {
		return WeeklySummaryResult{}, err
	}

	prompt := buildPeriodicPrompt(periodLabel, dutyRecords, idcOpsRecords, workTicketRecords, faultRecords)
	aiSummary, aiErr := callOpenAICompatible(ctx, configCenter, prompt)
	if aiErr != nil {
		aiSummary = fallbackSummary(periodLabel, dutyRecords, idcOpsRecords, workTicketRecords, faultRecords)
	}
	if strings.TrimSpace(aiSummary) == "" {
		aiSummary = fallbackSummary(periodLabel, dutyRecords, idcOpsRecords, workTicketRecords, faultRecords)
	}

	return WeeklySummaryResult{
		ReportType:            periodType,
		ReportTypeLabel:       periodLabel,
		PeriodStart:           periodStart,
		PeriodEnd:             periodEnd,
		GeneratedAt:           now,
		Summary:               aiSummary,
		DutyCount:             len(dutyRecords),
		IDCOpsTicketCount:     len(idcOpsRecords),
		WorkTicketCount:       len(workTicketRecords),
		NetworkFaultCount:     len(faultRecords),
		TicketCount:           len(idcOpsRecords),
		LegacyWorkTicketCount: len(workTicketRecords),
	}, nil
}

func normalizeReportType(reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "month":
		return "month"
	case "halfyear":
		return "halfyear"
	case "year":
		return "year"
	default:
		return "week"
	}
}

func reportWindow(now time.Time, reportType string) (time.Time, time.Time, string) {
	periodEnd := now
	switch reportType {
	case "month":
		return now.AddDate(0, -1, 0), periodEnd, "月报"
	case "halfyear":
		return now.AddDate(0, -6, 0), periodEnd, "半年报"
	case "year":
		return now.AddDate(-1, 0, 0), periodEnd, "年报"
	default:
		return now.AddDate(0, 0, -7), periodEnd, "周报"
	}
}

func fetchPeriodicRecords(db *gorm.DB, start, end time.Time) ([]DutyRecord, []IDCOpsRecord, []WorkTicketRecord, []NetworkFaultRecord, error) {
	startDate := start.Format("2006-01-02")
	endDate := end.Format("2006-01-02")

	var dutyRows []models.IdcDutyRecord
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&dutyRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var idcOpsRows []models.IDCOpsTicket
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&idcOpsRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var workRows []models.WorkTicket
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&workRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var faultRows []models.FaultRecord
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&faultRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var workTypes []models.WorkTicketType
	_ = db.Find(&workTypes).Error
	typeNameByID := make(map[uint]string, len(workTypes))
	for _, item := range workTypes {
		typeNameByID[item.ID] = item.Name
	}

	var faultTypes []models.FaultType
	_ = db.Find(&faultTypes).Error
	faultTypeNameByID := make(map[uint]string, len(faultTypes))
	for _, item := range faultTypes {
		faultTypeNameByID[item.ID] = item.Name
	}

	dutyRecords := make([]DutyRecord, 0, len(dutyRows))
	for _, row := range dutyRows {
		content := strings.TrimSpace(row.Tasks)
		if content == "" {
			content = fmt.Sprintf("运维值班: %s, 机房值班: %s", row.DutyOps, row.DutyIdc)
		}
		dutyRecords = append(dutyRecords, DutyRecord{Date: row.Date, Content: content})
	}

	idcOpsRecords := make([]IDCOpsRecord, 0, len(idcOpsRows))
	for _, row := range idcOpsRows {
		idcOpsRecords = append(idcOpsRecords, IDCOpsRecord{
			Date:                row.Date,
			VisitorOrganization: row.VisitorOrganization,
			VisitorCount:        row.VisitorCount,
			VisitorReason:       row.VisitorReason,
			CustomerService:     row.CustomerServicePerson,
		})
	}

	workTicketRecords := make([]WorkTicketRecord, 0, len(workRows))
	for _, row := range workRows {
		ticketTypeName := "-"
		if name, ok := typeNameByID[row.WorkTicketTypeID]; ok {
			ticketTypeName = name
		}
		workTicketRecords = append(workTicketRecords, WorkTicketRecord{
			Date:           row.Date,
			UserName:       row.UserName,
			TicketTypeName: ticketTypeName,
			OperationInfo:  row.OperationInfo,
			Status:         row.ProcessingStatus,
		})
	}

	faultRecords := make([]NetworkFaultRecord, 0, len(faultRows))
	for _, row := range faultRows {
		faultTypeName := "-"
		if name, ok := faultTypeNameByID[row.FaultTypeID]; ok {
			faultTypeName = name
		}
		faultRecords = append(faultRecords, NetworkFaultRecord{
			Date:          row.Date,
			UserName:      row.UserName,
			FaultSymptom:  row.FaultSymptom,
			Process:       row.ProcessingProcess,
			Status:        row.ProcessingStatus,
			FaultTypeName: faultTypeName,
		})
	}

	return dutyRecords, idcOpsRecords, workTicketRecords, faultRecords, nil
}

func buildPeriodicPrompt(periodLabel string, dutyRecords []DutyRecord, idcOpsRecords []IDCOpsRecord, workTicketRecords []WorkTicketRecord, faultRecords []NetworkFaultRecord) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("请根据以下%s数据，生成一份运维管理摘要。请包含以下三个部分：\n\n", periodLabel))
	b.WriteString("1. 核心工作成果\n")
	b.WriteString("2. 存在的主要问题\n")
	b.WriteString("3. 下一步建议\n\n")

	b.WriteString("IDC值班记录：\n")
	if len(dutyRecords) == 0 {
		b.WriteString("- 暂无 IDC 值班记录\n")
	} else {
		for _, record := range dutyRecords {
			b.WriteString(fmt.Sprintf("[%s] %s\n", record.Date.Format("2006-01-02"), trimForPreview(record.Content, 120)))
		}
	}

	b.WriteString("\nIDC运维工单：\n")
	if len(idcOpsRecords) == 0 {
		b.WriteString("- 暂无 IDC 运维工单\n")
	} else {
		for _, record := range idcOpsRecords {
			b.WriteString(fmt.Sprintf("[%s] %s 人数:%d 事由:%s\n",
				record.Date.Format("2006-01-02"),
				trimForPreview(record.VisitorOrganization, 60),
				record.VisitorCount,
				trimForPreview(record.VisitorReason, 100),
			))
		}
	}

	b.WriteString("\n网络运维工单：\n")
	if len(workTicketRecords) == 0 {
		b.WriteString("- 暂无网络运维工单\n")
	} else {
		for _, record := range workTicketRecords {
			b.WriteString(fmt.Sprintf("[%s] %s - %s\n  操作信息: %s\n  状态: %s\n",
				record.Date.Format("2006-01-02"),
				trimForPreview(record.UserName, 40),
				trimForPreview(record.TicketTypeName, 40),
				trimForPreview(record.OperationInfo, 100),
				record.Status,
			))
		}
	}

	b.WriteString("\n网络故障记录：\n")
	if len(faultRecords) == 0 {
		b.WriteString("- 暂无网络故障记录\n")
	} else {
		for _, record := range faultRecords {
			b.WriteString(fmt.Sprintf("[%s] %s 类型:%s 状态:%s\n  现象:%s\n",
				record.Date.Format("2006-01-02"),
				trimForPreview(record.UserName, 40),
				trimForPreview(record.FaultTypeName, 40),
				record.Status,
				trimForPreview(record.FaultSymptom, 100),
			))
		}
	}

	b.WriteString("\n请用简洁明了的中文回答，每个部分用清晰标题区分。")
	return b.String()
}

func callOpenAICompatible(ctx context.Context, configCenter *ConfigCenter, prompt string) (string, error) {
	if configCenter == nil {
		return "", fmt.Errorf("config center unavailable")
	}

	apiKey := strings.TrimSpace(configCenter.Get("AI_API_KEY", ""))
	baseURL := strings.TrimSpace(configCenter.Get("AI_API_BASE", ""))
	model := strings.TrimSpace(configCenter.Get("AI_MODEL", ""))
	if apiKey == "" || baseURL == "" || model == "" {
		return "", fmt.Errorf("missing AI config")
	}

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []map[string]string{
			{"role": "user", "content": prompt},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var parsed chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai api failed with status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ai api returned empty choices")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func fallbackSummary(periodLabel string, dutyRecords []DutyRecord, idcOpsRecords []IDCOpsRecord, workTicketRecords []WorkTicketRecord, faultRecords []NetworkFaultRecord) string {
	var b strings.Builder
	b.WriteString("一、核心工作成果\n")
	b.WriteString(fmt.Sprintf("- 本%s累计 IDC值班 %d 条，IDC运维工单 %d 条，网络运维工单 %d 条，网络故障记录 %d 条。\n", periodLabel, len(dutyRecords), len(idcOpsRecords), len(workTicketRecords), len(faultRecords)))
	b.WriteString("- 已完成值班巡检、接待登记、工单处理与故障闭环记录。\n\n")

	b.WriteString("二、存在的主要问题\n")
	statusCount := make(map[string]int)
	for _, item := range workTicketRecords {
		statusCount[strings.ToLower(strings.TrimSpace(item.Status))]++
	}
	for _, item := range faultRecords {
		statusCount["fault_"+strings.ToLower(strings.TrimSpace(item.Status))]++
	}
	if len(statusCount) == 0 {
		b.WriteString("- 当前样本数据较少，建议继续沉淀记录以形成稳定趋势。\n")
	} else {
		keys := make([]string, 0, len(statusCount))
		for key := range statusCount {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("- 状态 %s：%d 条。\n", key, statusCount[key]))
		}
	}
	b.WriteString("\n")

	b.WriteString("三、下一步建议\n")
	b.WriteString("- 强化交接摘要与提醒事项维护，确保跨班次无缝衔接。\n")
	b.WriteString("- 对进行中的工单和故障设置明确责任人、截止时间与复盘结论。\n")
	b.WriteString(fmt.Sprintf("- 固定输出%s并进行闭环复盘。", periodLabel))
	return b.String()
}

func trimForPreview(value string, max int) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "-"
	}
	if len([]rune(text)) <= max {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:max])) + "..."
}
