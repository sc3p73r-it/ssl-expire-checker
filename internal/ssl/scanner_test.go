package ssl

import (
	"testing"
	"time"
)

func TestClassifyExpiry(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		expiry  time.Time
		days    int
		want    string
	}{
		{"expired", now.Add(-time.Hour), 15, "expired"},
		{"expiring boundary", now.Add(15 * 24 * time.Hour), 15, "expiring"},
		{"valid just outside", now.Add(15*24*time.Hour + time.Second), 15, "valid"},
		{"valid far", now.Add(90 * 24 * time.Hour), 15, "valid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyExpiry(now, tc.expiry, tc.days)
			if got != tc.want {
				t.Fatalf("ClassifyExpiry(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"HTTPS://Example.COM/path", "example.com"},
		{"example.com:443", "example.com"},
		{"  cursor.com  ", "cursor.com"},
	}
	for _, tc := range cases {
		got, err := NormalizeHost(tc.in)
		if err != nil {
			t.Fatalf("NormalizeHost(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("NormalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
