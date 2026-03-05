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

type TicketRecord struct {
	CreatedAt time.Time
	Title     string
	Content   string
	Status    string
	Priority  string
}

type WorkTicketRecord struct {
	Date           time.Time
	UserName       string
	TicketTypeName string
	OperationInfo  string
	Status         string
}

type WeeklySummaryResult struct {
	PeriodStart     time.Time
	PeriodEnd       time.Time
	GeneratedAt     time.Time
	Summary         string
	DutyCount       int
	TicketCount     int
	WorkTicketCount int
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
	periodEnd := now
	periodStart := now.AddDate(0, 0, -7)

	dutyRecords, ticketRecords, workTicketRecords, err := fetchWeeklyRecords(db, periodStart, periodEnd)
	if err != nil {
		return WeeklySummaryResult{}, err
	}

	prompt := buildWeeklyPrompt(dutyRecords, ticketRecords, workTicketRecords)
	aiSummary, aiErr := callOpenAICompatible(ctx, configCenter, prompt)
	if aiErr != nil {
		aiSummary = fallbackSummary(dutyRecords, ticketRecords, workTicketRecords)
	}
	if strings.TrimSpace(aiSummary) == "" {
		aiSummary = fallbackSummary(dutyRecords, ticketRecords, workTicketRecords)
	}

	return WeeklySummaryResult{
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		GeneratedAt:     now,
		Summary:         aiSummary,
		DutyCount:       len(dutyRecords),
		TicketCount:     len(ticketRecords),
		WorkTicketCount: len(workTicketRecords),
	}, nil
}

func fetchWeeklyRecords(db *gorm.DB, start, end time.Time) ([]DutyRecord, []TicketRecord, []WorkTicketRecord, error) {
	startDate := start.Format("2006-01-02")
	endDate := end.Format("2006-01-02")

	var dutyRows []models.IdcDutyRecord
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&dutyRows).Error; err != nil {
		return nil, nil, nil, err
	}

	var ticketRows []models.Ticket
	if err := db.Where("created_at >= ? AND created_at <= ?", start, end).Order("created_at asc").Find(&ticketRows).Error; err != nil {
		return nil, nil, nil, err
	}

	var workRows []models.WorkTicket
	if err := db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date asc").Find(&workRows).Error; err != nil {
		return nil, nil, nil, err
	}

	var workTypes []models.WorkTicketType
	_ = db.Find(&workTypes).Error
	typeNameByID := make(map[uint]string, len(workTypes))
	for _, item := range workTypes {
		typeNameByID[item.ID] = item.Name
	}

	dutyRecords := make([]DutyRecord, 0, len(dutyRows))
	for _, row := range dutyRows {
		content := strings.TrimSpace(row.Tasks)
		if content == "" {
			content = fmt.Sprintf("运维值班: %s, 机房值班: %s", row.DutyOps, row.DutyIdc)
		}
		dutyRecords = append(dutyRecords, DutyRecord{
			Date:    row.Date,
			Content: content,
		})
	}

	ticketRecords := make([]TicketRecord, 0, len(ticketRows))
	for _, row := range ticketRows {
		ticketRecords = append(ticketRecords, TicketRecord{
			CreatedAt: row.CreatedAt,
			Title:     row.Title,
			Content:   row.Content,
			Status:    row.Status,
			Priority:  row.Priority,
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

	return dutyRecords, ticketRecords, workTicketRecords, nil
}

func buildWeeklyPrompt(dutyRecords []DutyRecord, ticketRecords []TicketRecord, workTicketRecords []WorkTicketRecord) string {
	var b strings.Builder
	b.WriteString("请根据以下过去7天的值班和工作记录，生成一份周报摘要。请包含以下三个部分：\n\n")
	b.WriteString("1. 核心工作成果\n")
	b.WriteString("2. 存在的主要问题\n")
	b.WriteString("3. 下一步建议\n\n")

	b.WriteString("值班记录：\n")
	if len(dutyRecords) == 0 {
		b.WriteString("- 暂无值班记录\n")
	} else {
		for _, record := range dutyRecords {
			b.WriteString(fmt.Sprintf("[%s] %s\n", record.Date.Format("2006-01-02"), trimForPreview(record.Content, 120)))
		}
	}

	b.WriteString("\n工单记录：\n")
	if len(ticketRecords) == 0 {
		b.WriteString("- 暂无工单记录\n")
	} else {
		for _, record := range ticketRecords {
			b.WriteString(fmt.Sprintf("[%s] %s - %s\n", record.CreatedAt.Format("2006-01-02"), trimForPreview(record.Title, 60), trimForPreview(record.Content, 100)))
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

	b.WriteString("\n请用简洁明了的中文回答，每个部分用清晰的标题区分。")
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
		MaxTokens:   800,
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

func fallbackSummary(dutyRecords []DutyRecord, ticketRecords []TicketRecord, workTicketRecords []WorkTicketRecord) string {
	var b strings.Builder
	b.WriteString("一、核心工作成果\n")
	b.WriteString(fmt.Sprintf("- 本周累计值班记录 %d 条，工单记录 %d 条，网络运维工单 %d 条。\n", len(dutyRecords), len(ticketRecords), len(workTicketRecords)))
	b.WriteString("- 已完成日常值班巡检、工单处理与问题跟踪。\n\n")

	b.WriteString("二、存在的主要问题\n")
	if len(ticketRecords) == 0 && len(workTicketRecords) == 0 {
		b.WriteString("- 当前工单样本较少，建议继续补充明细以便生成更准确趋势分析。\n")
	} else {
		statusCount := make(map[string]int)
		for _, item := range workTicketRecords {
			statusCount[strings.ToLower(strings.TrimSpace(item.Status))]++
		}
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
	b.WriteString("- 持续完善值班记录结构化填写，确保关键信息可统计。\n")
	b.WriteString("- 对进行中的工单设置明确截止时间与责任人。\n")
	b.WriteString("- 每周固定输出周报并复盘问题闭环情况。")
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
