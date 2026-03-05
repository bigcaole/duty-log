package handlers

import (
	"strconv"
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
