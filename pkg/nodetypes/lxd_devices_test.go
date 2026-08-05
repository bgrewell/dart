package nodetypes

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bgrewell/dart/pkg/ifaces"
)

func TestOptionValueToString(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{name: "string", value: "10", expected: "10"},
		{name: "bool", value: false, expected: "false"},
		{name: "int", value: 10, expected: "10"},
		{name: "int64", value: int64(4096), expected: "4096"},
		{name: "float from yaml integer", value: float64(10), expected: "10"},
		{name: "large float", value: float64(1000000), expected: "1000000"},
		{name: "fractional float", value: 1.5, expected: "1.5"},
		{name: "nil", value: nil, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := optionValueToString(tt.value); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuildDevices(t *testing.T) {
	devices := map[string]map[string]interface{}{
		"iso": {
			"type":          "disk",
			"source":        "testdata/boot.iso",
			"boot.priority": 10,
		},
	}

	built, err := buildDevices(devices, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Numeric YAML values must reach LXD as strings
	if built["iso"]["boot.priority"] != "10" {
		t.Errorf("expected boot.priority %q, got %q", "10", built["iso"]["boot.priority"])
	}

	// Relative sources are resolved so an ISO can be referenced by repository path
	expected, err := filepath.Abs("testdata/boot.iso")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if built["iso"]["source"] != expected {
		t.Errorf("expected source %q, got %q", expected, built["iso"]["source"])
	}
}

func TestBuildDevicesSourceHandling(t *testing.T) {
	tests := []struct {
		name         string
		device       map[string]interface{}
		resolvePaths bool
		expected     string
	}{
		{
			name:         "absolute source is left alone",
			device:       map[string]interface{}{"type": "disk", "source": "/srv/images/boot.iso"},
			resolvePaths: true,
			expected:     "/srv/images/boot.iso",
		},
		{
			name:         "pool backed volume is not a path",
			device:       map[string]interface{}{"type": "disk", "pool": "default", "source": "volume-name"},
			resolvePaths: true,
			expected:     "volume-name",
		},
		{
			name:         "remote server paths are left alone",
			device:       map[string]interface{}{"type": "disk", "source": "images/boot.iso"},
			resolvePaths: false,
			expected:     "images/boot.iso",
		},
		{
			name:         "non disk devices are not touched",
			device:       map[string]interface{}{"type": "nic", "source": "relative/thing"},
			resolvePaths: true,
			expected:     "relative/thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built, err := buildDevices(map[string]map[string]interface{}{"dev": tt.device}, tt.resolvePaths)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if built["dev"]["source"] != tt.expected {
				t.Errorf("expected source %q, got %q", tt.expected, built["dev"]["source"])
			}
		})
	}
}

func TestBuildDevicesRequiresType(t *testing.T) {
	_, err := buildDevices(map[string]map[string]interface{}{
		"iso": {"source": "/srv/images/boot.iso"},
	}, true)

	if err == nil {
		t.Fatal("expected an error for a device without a type")
	}
	if !contains(err.Error(), "missing a type") {
		t.Errorf("expected a missing type error, got %v", err)
	}
}

func TestEmptyInstance(t *testing.T) {
	tests := []struct {
		name     string
		opts     LxdNodeOpts
		expected bool
	}{
		{name: "image given", opts: LxdNodeOpts{Image: "24.04"}, expected: false},
		{name: "explicitly empty", opts: LxdNodeOpts{Empty: true}, expected: true},
		{name: "no image", opts: LxdNodeOpts{}, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.emptyInstance(); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestLxdNodeEmptyWithImageIsRejected(t *testing.T) {
	opts := map[string]interface{}{
		"empty":         true,
		"image":         "ubuntu:24.04",
		"instance_type": "virtual-machine",
	}

	_, err := NewLxdNode("test-node", ifaces.NodeOptions(&opts))
	if err == nil {
		t.Fatal("expected an error when both empty and image are set")
	}
	if !contains(err.Error(), "empty instances cannot specify an image") {
		t.Errorf("expected a validation error, got %v", err)
	}
}

func TestBootWaitReadinessConfig(t *testing.T) {
	// Unset values fall back to the defaults
	defaults := (&LxdBootWaitOpts{}).readinessConfig()
	if defaults.Timeout != 5*time.Minute {
		t.Errorf("expected default timeout of 5m, got %v", defaults.Timeout)
	}
	if defaults.PollInterval != 2*time.Second {
		t.Errorf("expected default interval of 2s, got %v", defaults.PollInterval)
	}

	config := (&LxdBootWaitOpts{Timeout: 1800, Interval: 15}).readinessConfig()
	if config.Timeout != 30*time.Minute {
		t.Errorf("expected timeout of 30m, got %v", config.Timeout)
	}
	if config.PollInterval != 15*time.Second {
		t.Errorf("expected interval of 15s, got %v", config.PollInterval)
	}
}

func TestBootWaitReadyCommand(t *testing.T) {
	command := (&LxdBootWaitOpts{ReadyCommand: "cat /etc/hostname"}).readyCommand("/bin/sh")
	expected := []string{"/bin/sh", "-c", "cat /etc/hostname"}
	for i := range expected {
		if command[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, command)
		}
	}

	// With no command configured, being able to run anything at all means ready
	fallback := (&LxdBootWaitOpts{}).readyCommand("/bin/bash")
	if fallback[2] != "true" {
		t.Errorf("expected fallback command %q, got %q", "true", fallback[2])
	}
}

func TestLxdNodeShell(t *testing.T) {
	tests := []struct {
		name     string
		options  LxdNodeOpts
		expected string
	}{
		{name: "default", options: LxdNodeOpts{}, expected: "/bin/bash"},
		{
			name:     "configured",
			options:  LxdNodeOpts{ExecOptions: map[string]interface{}{"shell": "/bin/sh"}},
			expected: "/bin/sh",
		},
		{
			name:     "non string value ignored",
			options:  LxdNodeOpts{ExecOptions: map[string]interface{}{"shell": 1}},
			expected: "/bin/bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &LxdNode{options: tt.options}
			if got := node.shell(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
