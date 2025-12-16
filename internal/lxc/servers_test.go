package lxc

import (
	"testing"
)

func TestGetUrlAndProtocol(t *testing.T) {
	tests := []struct {
		name        string
		alias       string
		wantUrl     string
		wantProto   string
		wantErr     bool
	}{
		{
			name:      "ubuntu alias",
			alias:     "ubuntu",
			wantUrl:   "https://cloud-images.ubuntu.com/releases",
			wantProto: "simplestreams",
			wantErr:   false,
		},
		{
			name:      "images alias",
			alias:     "images",
			wantUrl:   "https://images.linuxcontainers.org",
			wantProto: "simplestreams",
			wantErr:   false,
		},
		{
			name:      "lxc alias",
			alias:     "lxc",
			wantUrl:   "https://images.linuxcontainers.org",
			wantProto: "simplestreams",
			wantErr:   false,
		},
		{
			name:    "unknown alias",
			alias:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, proto, err := GetUrlAndProtocol(tt.alias)
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if url != tt.wantUrl {
				t.Errorf("Got url %s, want %s", url, tt.wantUrl)
			}
			
			if proto != tt.wantProto {
				t.Errorf("Got protocol %s, want %s", proto, tt.wantProto)
			}
		})
	}
}
