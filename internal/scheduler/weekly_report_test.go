package scheduler

import (
	"testing"
	"time"
)

func TestSplitFeishuContent(t *testing.T) {
	chunks := splitFeishuContent("", 10)
	if len(chunks) != 1 || chunks[0] != "-" {
		t.Fatalf("unexpected empty chunks: %#v", chunks)
	}

	chunks = splitFeishuContent("a\nb\nc", 3)
	if len(chunks) == 0 {
		t.Fatalf("expected chunks")
	}
}

func TestReportPeriodsForNow(t *testing.T) {
	now := time.Date(2026, time.June, 30, 17, 0, 0, 0, time.Local)
	periods := reportPeriodsForNow(now)
	if len(periods) == 0 {
		t.Fatalf("expected periods on 6/30")
	}
	hasMonth := false
	hasHalf := false
	for _, p := range periods {
		if p == "month" {
			hasMonth = true
		}
		if p == "halfyear" {
			hasHalf = true
		}
	}
	if !hasMonth || !hasHalf {
		t.Fatalf("unexpected periods: %#v", periods)
	}
}
