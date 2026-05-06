package notify

import (
	"strings"
	"testing"
)

func TestDetectKind(t *testing.T) {
	cases := []struct {
		url  string
		want Kind
	}{
		{"https://hooks.slack.com/services/XXX/YYY/ZZZ", KindSlack},
		{"https://discord.com/api/webhooks/123/token", KindDiscord},
		{"https://httpbin.org/post", KindGeneric},
	}
	for _, tc := range cases {
		if got := DetectKind(tc.url); got != tc.want {
			t.Fatalf("DetectKind(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestBuildPayloadSlack(t *testing.T) {
	raw, ct, err := buildPayload("https://hooks.slack.com/x", []ExpiringDomain{{URL: "a.com", Status: "expiring", Expiry: "2026-01-01"}})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "application/json" {
		t.Fatalf("content-type: %s", ct)
	}
	if !strings.Contains(string(raw), "a.com") {
		t.Fatalf("missing url in payload: %s", raw)
	}
}

func TestBuildPayloadDiscord(t *testing.T) {
	raw, _, err := buildPayload("https://discord.com/api/webhooks/1/t", []ExpiringDomain{{URL: "b.com", Status: "expired"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "embeds") {
		t.Fatalf("expected embeds: %s", raw)
	}
}
