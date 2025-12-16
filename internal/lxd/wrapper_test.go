package lxd

import (
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigToProfile(t *testing.T) {
	cfg := &config.LxdProfileConfig{
		Name:        "test-profile",
		Description: "A test profile",
		Config: map[string]string{
			"limits.cpu":    "2",
			"limits.memory": "4GB",
		},
		Devices: map[string]*config.LxdDeviceConfig{
			"root": {
				Type: "disk",
				Path: "/",
				Pool: "default",
			},
			"eth0": {
				Type: "nic",
				Name: "eth0",
				Opts: map[string]string{
					"network": "lxdbr0",
				},
			},
		},
	}

	profile := configToProfile(cfg)

	assert.Equal(t, "test-profile", profile.Name)
	assert.Equal(t, "A test profile", profile.Description)
	assert.Equal(t, "2", profile.Config["limits.cpu"])
	assert.Equal(t, "4GB", profile.Config["limits.memory"])
	assert.Equal(t, "disk", profile.Devices["root"].Type)
	assert.Equal(t, "/", profile.Devices["root"].Path)
	assert.Equal(t, "default", profile.Devices["root"].Pool)
	assert.Equal(t, "nic", profile.Devices["eth0"].Type)
	assert.Equal(t, "eth0", profile.Devices["eth0"].Name)
	assert.Equal(t, "lxdbr0", profile.Devices["eth0"].Opts["network"])
}

func TestConfigToProfileNilDevices(t *testing.T) {
	cfg := &config.LxdProfileConfig{
		Name:        "simple-profile",
		Description: "A simple profile without devices",
		Config: map[string]string{
			"limits.cpu": "1",
		},
		Devices: nil,
	}

	profile := configToProfile(cfg)

	assert.Equal(t, "simple-profile", profile.Name)
	assert.Equal(t, "A simple profile without devices", profile.Description)
	assert.Equal(t, "1", profile.Config["limits.cpu"])
	assert.NotNil(t, profile.Devices)
	assert.Len(t, profile.Devices, 0)
}

func TestWithImageServer(t *testing.T) {
	cfg := &InstanceConfig{}
	opt := WithImageServer("https://custom.server.com")
	opt(cfg)
	assert.Equal(t, "https://custom.server.com", cfg.ImageServer)
}

func TestWithProtocol(t *testing.T) {
	cfg := &InstanceConfig{}
	opt := WithProtocol("lxd")
	opt(cfg)
	assert.Equal(t, "lxd", cfg.Protocol)
}

func TestWithProfiles(t *testing.T) {
	cfg := &InstanceConfig{}
	profiles := []string{"default", "custom"}
	opt := WithProfiles(profiles)
	opt(cfg)
	assert.Equal(t, profiles, cfg.Profiles)
}

func TestWithConfig(t *testing.T) {
	cfg := &InstanceConfig{}
	config := map[string]string{"security.nesting": "true"}
	opt := WithConfig(config)
	opt(cfg)
	assert.Equal(t, config, cfg.Config)
}

func TestWithDevices(t *testing.T) {
	cfg := &InstanceConfig{}
	devices := map[string]Device{
		"root": {Type: "disk", Path: "/", Pool: "default"},
	}
	opt := WithDevices(devices)
	opt(cfg)
	assert.Equal(t, devices, cfg.Devices)
}

func TestWithEphemeral(t *testing.T) {
	cfg := &InstanceConfig{}
	opt := WithEphemeral(true)
	opt(cfg)
	assert.True(t, cfg.Ephemeral)
}
