package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf/v2"
)

type WeeklyReportPDFData struct {
	ReportTypeLabel   string
	PeriodStart       time.Time
	PeriodEnd         time.Time
	GeneratedAt       time.Time
	DutyCount         int
	IDCOpsTicketCount int
	WorkTicketCount   int
	NetworkFaultCount int
	Summary           string
}

func GenerateWeeklyReportPDF(data WeeklyReportPDFData) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()

	fontFamily := "Helvetica"
	if fontPath, ok := findFontForPDF(); ok {
		pdf.SetFontLocation(filepath.Dir(fontPath))
		pdf.AddUTF8Font("custom", "", filepath.Base(fontPath))
		fontFamily = "custom"
	}

	pdf.SetFont(fontFamily, "B", 18)
	title := "Report"
	if strings.TrimSpace(data.ReportTypeLabel) != "" {
		title = data.ReportTypeLabel
	}
	pdf.CellFormat(0, 10, safePDFText(fontFamily, title), "", 1, "L", false, 0, "")

	pdf.SetFont(fontFamily, "", 11)
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("报告周期: %s ~ %s", data.PeriodStart.Format("2006-01-02"), data.PeriodEnd.Format("2006-01-02"))), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("生成时间: %s", data.GeneratedAt.Format("2006-01-02 15:04:05"))), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(fontFamily, "B", 13)
	pdf.CellFormat(0, 8, safePDFText(fontFamily, "统计概览"), "", 1, "L", false, 0, "")
	pdf.SetFont(fontFamily, "", 11)
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("- IDC 值班记录数: %d", data.DutyCount)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("- IDC 运维工单数: %d", data.IDCOpsTicketCount)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("- 网络运维工单数: %d", data.WorkTicketCount)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, safePDFText(fontFamily, fmt.Sprintf("- 网络故障记录数: %d", data.NetworkFaultCount)), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(fontFamily, "B", 13)
	pdf.CellFormat(0, 8, safePDFText(fontFamily, "AI 周报摘要"), "", 1, "L", false, 0, "")
	pdf.SetFont(fontFamily, "", 11)
	pdf.MultiCell(0, 6, safePDFText(fontFamily, strings.TrimSpace(data.Summary)), "", "L", false)

	pdf.SetY(-10)
	pdf.SetFont(fontFamily, "", 9)
	pdf.CellFormat(0, 6, safePDFText(fontFamily, "Duty-Log-System"), "", 0, "R", false, 0, "")

	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("weekly-report-%s.pdf", time.Now().Format("20060102-150405")))
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}

func findFontForPDF() (string, bool) {
	var candidates []string

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, "fonts", "NotoSansSC-Regular.ttf"),
			filepath.Join(execDir, "fonts", "simhei.ttf"),
			filepath.Join(execDir, "fonts", "msyh.ttf"),
		)
	}

	candidates = append(candidates,
		filepath.Join("fonts", "NotoSansSC-Regular.ttf"),
		filepath.Join("fonts", "simhei.ttf"),
		filepath.Join("fonts", "msyh.ttf"),
		filepath.Join("/app/fonts", "NotoSansSC-Regular.ttf"),
		filepath.Join("/app/fonts", "simhei.ttf"),
		filepath.Join("/app/fonts", "msyh.ttf"),
	)

	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func safePDFText(fontFamily, text string) string {
	if fontFamily == "custom" {
		return text
	}
	// Core PDF fonts do not support Unicode well. Fall back to ASCII-safe content.
	var b strings.Builder
	for _, r := range text {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		} else if r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		} else {
			b.WriteRune('?')
		}
	}
	return b.String()
}
