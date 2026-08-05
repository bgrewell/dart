package nodetypes

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/canonical/lxd/shared/api"
)

func TestParseTrustToken(t *testing.T) {
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	encoded := (&api.CertificateAddToken{
		ClientName:  "dart-client",
		Fingerprint: "57bb0ff4340b5bb28517e062023101adf788c37846dc8b619eb2c3cb4ef29436",
		Addresses:   []string{"10.0.0.1:8443"},
		Secret:      "2b2284d44db32675923fe0d2020477e0e9be11801ff70c435e032b97028c35cd",
		ExpiresAt:   expires,
	}).String()

	token, err := parseTrustToken(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.ClientName != "dart-client" {
		t.Errorf("expected client name %q, got %q", "dart-client", token.ClientName)
	}
	if !token.ExpiresAt.Equal(expires) {
		t.Errorf("expected expiry %v, got %v", expires, token.ExpiresAt)
	}
}

func TestParseTrustTokenErrors(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		errorMsg string
	}{
		{
			name:     "not base64",
			token:    "this is not a token",
			errorMsg: "not valid base64",
		},
		{
			name:     "base64 but not json",
			token:    base64.StdEncoding.EncodeToString([]byte("not json at all")),
			errorMsg: "not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTrustToken(tt.token)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestTrustTokenExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		token    *api.CertificateAddToken
		expected bool
	}{
		{
			name:     "no token",
			token:    nil,
			expected: false,
		},
		{
			name:     "no expiry never expires",
			token:    &api.CertificateAddToken{},
			expected: false,
		},
		{
			name:     "expires later",
			token:    &api.CertificateAddToken{ExpiresAt: now.Add(time.Hour)},
			expected: false,
		},
		{
			name:     "just expired stays usable within the skew allowance",
			token:    &api.CertificateAddToken{ExpiresAt: now.Add(-5 * time.Second)},
			expected: false,
		},
		{
			name:     "expired well beyond the skew allowance",
			token:    &api.CertificateAddToken{ExpiresAt: now.Add(-time.Hour)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trustTokenExpired(tt.token, now); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

// Expired and malformed tokens are rejected before any connection is attempted,
// so these cases return quickly without reaching the server.
func TestLxdNodeRejectsUnusableTrustTokens(t *testing.T) {
	expired := (&api.CertificateAddToken{
		ClientName: "dart-client",
		ExpiresAt:  time.Now().Add(-time.Hour),
	}).String()

	tests := []struct {
		name     string
		token    string
		errorMsg string
	}{
		{
			name:     "expired token",
			token:    expired,
			errorMsg: "trust token expired at",
		},
		{
			name:     "malformed token",
			token:    "not-a-real-token",
			errorMsg: "invalid trust token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"trust_token": tt.token,
				"image":       "ubuntu:24.04",
			}

			_, err := NewLxdNode("test-node", ifaces.NodeOptions(&opts))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}
