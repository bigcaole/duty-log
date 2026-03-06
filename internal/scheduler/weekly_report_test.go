package scheduler

import "testing"

func TestParseCSVEmails(t *testing.T) {
	emails := parseCSVEmails(" a@example.com, b@example.com ,, ")
	if len(emails) != 2 {
		t.Fatalf("unexpected email count: %d", len(emails))
	}
	if emails[0] != "a@example.com" || emails[1] != "b@example.com" {
		t.Fatalf("unexpected emails: %#v", emails)
	}
}

func TestTrimWeeklyMessage(t *testing.T) {
	if got := trimWeeklyMessage("", 10); got != "-" {
		t.Fatalf("empty summary should fallback to -, got=%q", got)
	}
	if got := trimWeeklyMessage("123456", 4); got != "1234..." {
		t.Fatalf("expected truncation, got=%q", got)
	}
}
