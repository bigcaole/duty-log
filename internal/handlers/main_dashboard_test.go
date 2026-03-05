package handlers

import "testing"

func TestDashboardScopeLabel(t *testing.T) {
	if got := dashboardScopeLabel(true); got != "全局数据" {
		t.Fatalf("unexpected admin scope label: %q", got)
	}
	if got := dashboardScopeLabel(false); got != "我的数据" {
		t.Fatalf("unexpected user scope label: %q", got)
	}
}
