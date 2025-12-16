package lxd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSubnetMask(t *testing.T) {
	tests := []struct {
		name     string
		subnet   string
		expected string
	}{
		{
			name:     "standard /24 subnet",
			subnet:   "10.0.0.0/24",
			expected: "24",
		},
		{
			name:     "/16 subnet",
			subnet:   "172.16.0.0/16",
			expected: "16",
		},
		{
			name:     "/8 subnet",
			subnet:   "10.0.0.0/8",
			expected: "8",
		},
		{
			name:     "no subnet mask",
			subnet:   "10.0.0.0",
			expected: "24", // default
		},
		{
			name:     "empty string",
			subnet:   "",
			expected: "24", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSubnetMask(tt.subnet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstanceType(t *testing.T) {
	assert.Equal(t, InstanceType("container"), InstanceTypeContainer)
	assert.Equal(t, InstanceType("virtual-machine"), InstanceTypeVM)
}

func TestInstanceConfig(t *testing.T) {
	config := &InstanceConfig{
		Name:        "test-instance",
		Type:        InstanceTypeContainer,
		Image:       "ubuntu/22.04",
		ImageServer: "https://images.linuxcontainers.org",
		Protocol:    "simplestreams",
		Profiles:    []string{"default"},
		Config: map[string]string{
			"security.nesting": "true",
		},
		Devices: map[string]Device{
			"root": {
				Type: "disk",
				Path: "/",
				Pool: "default",
			},
		},
		Ephemeral: false,
	}

	assert.Equal(t, "test-instance", config.Name)
	assert.Equal(t, InstanceTypeContainer, config.Type)
	assert.Equal(t, "ubuntu/22.04", config.Image)
	assert.Equal(t, "https://images.linuxcontainers.org", config.ImageServer)
	assert.Equal(t, "simplestreams", config.Protocol)
	assert.Equal(t, []string{"default"}, config.Profiles)
	assert.Equal(t, "true", config.Config["security.nesting"])
	assert.Equal(t, "disk", config.Devices["root"].Type)
	assert.Equal(t, "/", config.Devices["root"].Path)
	assert.Equal(t, "default", config.Devices["root"].Pool)
	assert.False(t, config.Ephemeral)
}

func TestProfile(t *testing.T) {
	profile := &Profile{
		Name:        "test-profile",
		Description: "A test profile",
		Config: map[string]string{
			"limits.cpu": "2",
		},
		Devices: map[string]Device{
			"eth0": {
				Type: "nic",
				Name: "eth0",
				Opts: map[string]string{
					"network": "lxdbr0",
				},
			},
		},
	}

	assert.Equal(t, "test-profile", profile.Name)
	assert.Equal(t, "A test profile", profile.Description)
	assert.Equal(t, "2", profile.Config["limits.cpu"])
	assert.Equal(t, "nic", profile.Devices["eth0"].Type)
	assert.Equal(t, "eth0", profile.Devices["eth0"].Name)
	assert.Equal(t, "lxdbr0", profile.Devices["eth0"].Opts["network"])
}

func TestNetwork(t *testing.T) {
	network := &Network{
		Name:    "test-network",
		Type:    "bridge",
		Subnet:  "10.0.0.0/24",
		Gateway: "10.0.0.1",
	}

	assert.Equal(t, "test-network", network.Name)
	assert.Equal(t, "bridge", network.Type)
	assert.Equal(t, "10.0.0.0/24", network.Subnet)
	assert.Equal(t, "10.0.0.1", network.Gateway)
}

func TestInstanceFilter(t *testing.T) {
	filter := &InstanceFilter{
		Name:   "test-instance",
		Status: "Running",
		Type:   "container",
	}

	assert.Equal(t, "test-instance", filter.Name)
	assert.Equal(t, "Running", filter.Status)
	assert.Equal(t, "container", filter.Type)
}

func TestDevice(t *testing.T) {
	device := &Device{
		Type: "disk",
		Path: "/mnt/data",
		Pool: "default",
		Name: "data",
		Opts: map[string]string{
			"size": "10GB",
		},
	}

	assert.Equal(t, "disk", device.Type)
	assert.Equal(t, "/mnt/data", device.Path)
	assert.Equal(t, "default", device.Pool)
	assert.Equal(t, "data", device.Name)
	assert.Equal(t, "10GB", device.Opts["size"])
}

func TestImage(t *testing.T) {
	image := &Image{
		Alias:    "ubuntu/22.04",
		Server:   "https://images.linuxcontainers.org",
		Protocol: "simplestreams",
	}

	assert.Equal(t, "ubuntu/22.04", image.Alias)
	assert.Equal(t, "https://images.linuxcontainers.org", image.Server)
	assert.Equal(t, "simplestreams", image.Protocol)
}

func TestConnectionOptions(t *testing.T) {
	opts := &ConnectionOptions{
		UnixSocket:   "/var/lib/lxd/unix.socket",
		HTTPSAddress: "https://lxd.example.com:8443",
		ClientCert:   "/path/to/client.crt",
		ClientKey:    "/path/to/client.key",
		ServerCert:   "/path/to/server.crt",
	}

	assert.Equal(t, "/var/lib/lxd/unix.socket", opts.UnixSocket)
	assert.Equal(t, "https://lxd.example.com:8443", opts.HTTPSAddress)
	assert.Equal(t, "/path/to/client.crt", opts.ClientCert)
	assert.Equal(t, "/path/to/client.key", opts.ClientKey)
	assert.Equal(t, "/path/to/server.crt", opts.ServerCert)
}
