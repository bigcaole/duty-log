package handlers

import "testing"

func TestTableNameLabel(t *testing.T) {
	if got := tableNameLabel("duty_logs"); got != "值班日志" {
		t.Fatalf("unexpected duty_logs label: %s", got)
	}
	if got := tableNameLabel("unknown_table"); got != "unknown_table" {
		t.Fatalf("unexpected fallback label: %s", got)
	}
}

func TestTrimDashboardText(t *testing.T) {
	if got := trimDashboardText("", 20); got != "-" {
		t.Fatalf("empty text should fallback to -, got=%q", got)
	}
	if got := trimDashboardText("a\nb\rc", 20); got != "a b c" {
		t.Fatalf("line breaks should normalize, got=%q", got)
	}
	if got := trimDashboardText("123456", 4); got != "1234..." {
		t.Fatalf("expected truncation, got=%q", got)
	}
}
