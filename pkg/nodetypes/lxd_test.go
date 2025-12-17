package nodetypes

import (
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
)

func TestLxdNodeRemoteValidation(t *testing.T) {
	tests := []struct {
		name        string
		opts        map[string]interface{}
		shouldError bool
		errorMsg    string
	}{
		{
			name: "local connection - no remote_addr",
			opts: map[string]interface{}{
				"image":         "ubuntu:24.04",
				"instance_type": "container",
			},
			shouldError: false,
		},
		{
			name: "remote connection with certificates",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"client_cert": "/path/to/cert.crt",
				"client_key":  "/path/to/key.key",
				"image":       "ubuntu:24.04",
			},
			shouldError: false,
		},
		{
			name: "remote connection with skip_verify",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"skip_verify": true,
				"image":       "ubuntu:24.04",
			},
			shouldError: false,
		},
		{
			name: "remote connection missing certificates without skip_verify",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"image":       "ubuntu:24.04",
			},
			shouldError: true,
			errorMsg:    "remote LXD connection requires either trust_token OR",
		},
		{
			name: "remote connection with only client_cert",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"client_cert": "/path/to/cert.crt",
				"image":       "ubuntu:24.04",
			},
			shouldError: true,
			errorMsg:    "remote LXD connection requires either trust_token OR",
		},
		{
			name: "remote connection with only client_key",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"client_key":  "/path/to/key.key",
				"image":       "ubuntu:24.04",
			},
			shouldError: true,
			errorMsg:    "remote LXD connection requires",
		},
		{
			name: "remote connection with trust token",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"trust_token": "eyJjbGllbnRfbmFtZSI6InRlc3QifQ==",
				"image":       "ubuntu:24.04",
			},
			shouldError: false, // Will fail on actual connection, but validation should pass
		},
		{
			name: "remote connection with both trust token and certificates",
			opts: map[string]interface{}{
				"remote_addr": "https://10.0.0.1:8443",
				"trust_token": "eyJjbGllbnRfbmFtZSI6InRlc3QifQ==",
				"client_cert": "/path/to/cert.crt",
				"client_key":  "/path/to/key.key",
				"image":       "ubuntu:24.04",
			},
			shouldError: false, // Trust token takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLxdNode("test-node", ifaces.NodeOptions(&tt.opts))

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				// For success cases, we expect connection errors since we're not actually connecting
				// to a real LXD server, but we should get past the validation stage
				if err != nil && contains(err.Error(), "remote LXD connection requires client_cert and client_key") {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
