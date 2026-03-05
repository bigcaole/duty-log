package handlers

import "testing"

func TestParseBoolQuery(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"YES", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"", false},
	}

	for _, tc := range cases {
		got := parseBoolQuery(tc.in)
		if got != tc.want {
			t.Fatalf("parseBoolQuery(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMaskBackupPassword(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abc", "****"},
		{"abcd", "****"},
		{"abcde", "a****e"},
		{"abcdefgh", "a****h"},
		{"abcdefghi", "ab****hi"},
	}

	for _, tc := range cases {
		got := maskBackupPassword(tc.in)
		if got != tc.want {
			t.Fatalf("maskBackupPassword(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
