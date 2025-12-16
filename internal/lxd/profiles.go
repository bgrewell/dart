package lxd

import (
	"context"
	"fmt"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ListProfiles returns a list of all profiles
func ListProfiles(ctx context.Context, server lxd.InstanceServer) ([]api.Profile, error) {
	profiles, err := server.GetProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	return profiles, nil
}

// GetProfile retrieves a specific profile by name
func GetProfile(ctx context.Context, server lxd.InstanceServer, name string) (*api.Profile, string, error) {
	profile, etag, err := server.GetProfile(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get profile %s: %w", name, err)
	}
	return profile, etag, nil
}

// CreateProfile creates a new profile
func CreateProfile(ctx context.Context, server lxd.InstanceServer, profile *Profile) error {
	// Build device map
	devices := make(map[string]map[string]string)
	for name, dev := range profile.Devices {
		deviceMap := map[string]string{
			"type": dev.Type,
		}
		if dev.Path != "" {
			deviceMap["path"] = dev.Path
		}
		if dev.Pool != "" {
			deviceMap["pool"] = dev.Pool
		}
		if dev.Name != "" {
			deviceMap["name"] = dev.Name
		}
		for k, v := range dev.Opts {
			deviceMap[k] = v
		}
		devices[name] = deviceMap
	}

	req := api.ProfilesPost{
		Name: profile.Name,
		ProfilePut: api.ProfilePut{
			Description: profile.Description,
			Config:      profile.Config,
			Devices:     devices,
		},
	}

	err := server.CreateProfile(req)
	if err != nil {
		return fmt.Errorf("failed to create profile %s: %w", profile.Name, err)
	}

	return nil
}

// UpdateProfile updates an existing profile
func UpdateProfile(ctx context.Context, server lxd.InstanceServer, name string, config map[string]string, etag string) error {
	profile, currentEtag, err := server.GetProfile(name)
	if err != nil {
		return fmt.Errorf("failed to get profile %s for update: %w", name, err)
	}

	if etag != "" {
		currentEtag = etag
	}

	// Merge config
	for k, v := range config {
		profile.Config[k] = v
	}

	err = server.UpdateProfile(name, profile.Writable(), currentEtag)
	if err != nil {
		return fmt.Errorf("failed to update profile %s: %w", name, err)
	}

	return nil
}

// DeleteProfile deletes a profile
func DeleteProfile(ctx context.Context, server lxd.InstanceServer, name string) error {
	err := server.DeleteProfile(name)
	if err != nil {
		return fmt.Errorf("failed to delete profile %s: %w", name, err)
	}
	return nil
}

// AddDeviceToProfile adds a device to a profile
func AddDeviceToProfile(ctx context.Context, server lxd.InstanceServer, profileName, deviceName string, device *Device) error {
	profile, etag, err := server.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get profile %s: %w", profileName, err)
	}

	if profile.Devices == nil {
		profile.Devices = make(map[string]map[string]string)
	}

	deviceMap := map[string]string{
		"type": device.Type,
	}
	if device.Path != "" {
		deviceMap["path"] = device.Path
	}
	if device.Pool != "" {
		deviceMap["pool"] = device.Pool
	}
	if device.Name != "" {
		deviceMap["name"] = device.Name
	}
	for k, v := range device.Opts {
		deviceMap[k] = v
	}

	profile.Devices[deviceName] = deviceMap

	err = server.UpdateProfile(profileName, profile.Writable(), etag)
	if err != nil {
		return fmt.Errorf("failed to add device %s to profile %s: %w", deviceName, profileName, err)
	}

	return nil
}

// RemoveDeviceFromProfile removes a device from a profile
func RemoveDeviceFromProfile(ctx context.Context, server lxd.InstanceServer, profileName, deviceName string) error {
	profile, etag, err := server.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get profile %s: %w", profileName, err)
	}

	if _, exists := profile.Devices[deviceName]; !exists {
		return fmt.Errorf("device %s not found in profile %s", deviceName, profileName)
	}

	delete(profile.Devices, deviceName)

	err = server.UpdateProfile(profileName, profile.Writable(), etag)
	if err != nil {
		return fmt.Errorf("failed to remove device %s from profile %s: %w", deviceName, profileName, err)
	}

	return nil
}
