package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Kind describes which webhook format to use.
type Kind int

const (
	KindGeneric Kind = iota
	KindSlack
	KindDiscord
)

// DetectKind picks Slack vs Discord vs generic from the webhook URL.
func DetectKind(webhookURL string) Kind {
	u := strings.ToLower(webhookURL)
	switch {
	case strings.Contains(u, "hooks.slack.com"):
		return KindSlack
	case strings.Contains(u, "discord.com/api/webhooks"):
		return KindDiscord
	default:
		return KindGeneric
	}
}

// ExpiringDomain is a line item in a digest notification.
type ExpiringDomain struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Expiry string `json:"expiry,omitempty"`
}

// SendDigest posts a single payload listing domains that need attention.
func SendDigest(ctx context.Context, webhookURL string, items []ExpiringDomain) error {
	if webhookURL == "" || len(items) == 0 {
		return nil
	}
	body, contentType, err := buildPayload(webhookURL, items)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

func buildPayload(webhookURL string, items []ExpiringDomain) ([]byte, string, error) {
	switch DetectKind(webhookURL) {
	case KindSlack:
		return slackBlocks(items)
	case KindDiscord:
		return discordEmbed(items)
	default:
		var b strings.Builder
		b.WriteString("SSL Expiry Checker digest:\n")
		for _, it := range items {
			b.WriteString(fmt.Sprintf("- %s [%s] expires=%s\n", it.URL, it.Status, it.Expiry))
		}
		raw, err := json.Marshal(map[string]string{"text": b.String()})
		return raw, "application/json", err
	}
}

func slackBlocks(items []ExpiringDomain) ([]byte, string, error) {
	var blocks []map[string]any
	blocks = append(blocks, map[string]any{
		"type": "header",
		"text": map[string]string{"type": "plain_text", "text": "SSL certificate alerts"},
	})
	for _, it := range items {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s* — `%s`%s", it.URL, it.Status, slackExpiry(it.Expiry)),
			},
		})
	}
	payload := map[string]any{"blocks": blocks}
	raw, err := json.Marshal(payload)
	return raw, "application/json", err
}

func slackExpiry(exp string) string {
	if exp == "" {
		return ""
	}
	return fmt.Sprintf(" — expires _%s_", exp)
}

func discordEmbed(items []ExpiringDomain) ([]byte, string, error) {
	var fields []map[string]any
	for _, it := range items {
		fields = append(fields, map[string]any{
			"name":   it.URL,
			"value":  fmt.Sprintf("Status: **%s**\nExpiry: %s", it.Status, it.Expiry),
			"inline": false,
		})
	}
	payload := map[string]any{
		"embeds": []map[string]any{{
			"title":       "SSL certificate alerts",
			"description": "The following domains need attention.",
			"color":       0xe74c3c,
			"fields":      fields,
		}},
	}
	raw, err := json.Marshal(payload)
	return raw, "application/json", err
}
