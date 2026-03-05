package handlers

import "testing"

func TestCanUseGlobalWeeklySummary(t *testing.T) {
	if !canUseGlobalWeeklySummary(true) {
		t.Fatalf("admin should be able to use global weekly summary")
	}
	if canUseGlobalWeeklySummary(false) {
		t.Fatalf("non-admin should not be able to use global weekly summary")
	}
}

func TestStatisticsUserCountLabel(t *testing.T) {
	if got := statisticsUserCountLabel(true); got != "系统用户" {
		t.Fatalf("unexpected admin label: %q", got)
	}
	if got := statisticsUserCountLabel(false); got != "我的账号" {
		t.Fatalf("unexpected non-admin label: %q", got)
	}
}
