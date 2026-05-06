package ssl

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

const dialTimeout = 10 * time.Second

// ScanResult holds certificate metadata and classification for a host.
type ScanResult struct {
	Issuer        *string
	Expiry        *time.Time
	Status        string
	LastError     *string
	LastChecked   time.Time
}

// ClassifyExpiry maps an expiry time to dashboard status.
func ClassifyExpiry(now, expiry time.Time, thresholdDays int) string {
	if expiry.Before(now) {
		return "expired"
	}
	if expiry.Sub(now) <= time.Duration(thresholdDays)*24*time.Hour {
		return "expiring"
	}
	return "valid"
}

// NormalizeHost strips schemes/ports/path and returns host for TLS dial.
func NormalizeHost(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty host")
	}
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.IndexAny(s, "/?#"); idx >= 0 {
		s = s[:idx]
	}
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		host = s
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("invalid host")
	}
	return strings.ToLower(host), nil
}

// Scan connects to host:443 and reads the leaf certificate.
func Scan(host string, thresholdDays int) ScanResult {
	now := time.Now().UTC()
	res := ScanResult{LastChecked: now}

	h, err := NormalizeHost(host)
	if err != nil {
		msg := err.Error()
		res.Status = "error"
		res.LastError = &msg
		return res
	}

	d := net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(&d, "tcp", net.JoinHostPort(h, "443"), &tls.Config{
		ServerName:         h,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		msg := err.Error()
		res.Status = "error"
		res.LastError = &msg
		return res
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		msg := "no peer certificates"
		res.Status = "error"
		res.LastError = &msg
		return res
	}
	cert := state.PeerCertificates[0]
	issuer := cert.Issuer.String()
	exp := cert.NotAfter.UTC()
	res.Issuer = &issuer
	res.Expiry = &exp
	res.Status = ClassifyExpiry(now, exp, thresholdDays)
	return res
}
