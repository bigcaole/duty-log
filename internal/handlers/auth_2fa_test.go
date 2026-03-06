package handlers

import (
	"bytes"
	htemplate "html/template"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSessionIntValue(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{nil, 0},
		{3, 3},
		{int64(9), 9},
		{uint(7), 7},
		{float64(5), 5},
		{"12", 12},
		{" 15 ", 15},
		{"invalid", 0},
	}

	for _, tc := range cases {
		got := sessionIntValue(tc.in)
		if got != tc.want {
			t.Fatalf("sessionIntValue(%v)=%d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSessionUnixTimeValue(t *testing.T) {
	now := time.Now().Unix()
	got := sessionUnixTimeValue(strconv.FormatInt(now, 10))
	if got.IsZero() {
		t.Fatalf("expected non-zero time")
	}
	if got.Unix() != now {
		t.Fatalf("unexpected unix value: got=%d want=%d", got.Unix(), now)
	}

	zero := sessionUnixTimeValue("0")
	if !zero.IsZero() {
		t.Fatalf("expected zero time for 0")
	}
}

func TestSafeTemplateDataImageURL(t *testing.T) {
	raw := "data:image/png;base64,AAAA"
	if got := string(safeTemplateDataImageURL(raw)); got != raw {
		t.Fatalf("expected data image url to pass through, got=%q", got)
	}

	if got := string(safeTemplateDataImageURL("javascript:alert(1)")); got != "" {
		t.Fatalf("expected non-image data url to be blocked, got=%q", got)
	}
}

func TestTemplateDataURLEscapingBehavior(t *testing.T) {
	tpl := htemplate.Must(htemplate.New("x").Parse(`<img src="{{.}}">`))

	var plain bytes.Buffer
	if err := tpl.Execute(&plain, "data:image/png;base64,AAAA"); err != nil {
		t.Fatalf("execute template with plain string failed: %v", err)
	}
	if !strings.Contains(plain.String(), "#ZgotmplZ") {
		t.Fatalf("expected plain data url string to be sanitized, got=%s", plain.String())
	}

	var safe bytes.Buffer
	if err := tpl.Execute(&safe, safeTemplateDataImageURL("data:image/png;base64,AAAA")); err != nil {
		t.Fatalf("execute template with safe url failed: %v", err)
	}
	if strings.Contains(safe.String(), "#ZgotmplZ") {
		t.Fatalf("expected safe template URL to avoid sanitization, got=%s", safe.String())
	}
}
