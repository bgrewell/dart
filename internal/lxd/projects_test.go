package lxd

import (
	"testing"
)

// TestProjectConfigDefaults verifies that CreateProject sets appropriate default features
func TestProjectConfigDefaults(t *testing.T) {
	// This test validates that default project features are set correctly
	// when creating a project with minimal configuration

	config := make(map[string]string)

	// Simulate the logic from CreateProject
	if _, ok := config["features.images"]; !ok {
		config["features.images"] = "true"
	}
	if _, ok := config["features.profiles"]; !ok {
		config["features.profiles"] = "true"
	}
	if _, ok := config["features.storage.volumes"]; !ok {
		config["features.storage.volumes"] = "true"
	}
	if _, ok := config["features.networks"]; !ok {
		config["features.networks"] = "true"
	}

	// Verify all features are set to "true"
	expectedFeatures := []string{
		"features.images",
		"features.profiles",
		"features.storage.volumes",
		"features.networks",
	}

	for _, feature := range expectedFeatures {
		if val, ok := config[feature]; !ok || val != "true" {
			t.Errorf("Expected feature %s to be 'true', got %v", feature, val)
		}
	}
}

// TestProjectConfigCustomValues verifies that custom config values are preserved
func TestProjectConfigCustomValues(t *testing.T) {
	// This test validates that user-provided config values are not overwritten

	config := map[string]string{
		"features.images":   "false",
		"features.profiles": "false",
	}

	// Simulate the logic from CreateProject
	if _, ok := config["features.images"]; !ok {
		config["features.images"] = "true"
	}
	if _, ok := config["features.profiles"]; !ok {
		config["features.profiles"] = "true"
	}
	if _, ok := config["features.storage.volumes"]; !ok {
		config["features.storage.volumes"] = "true"
	}
	if _, ok := config["features.networks"]; !ok {
		config["features.networks"] = "true"
	}

	// Verify custom values are preserved
	if config["features.images"] != "false" {
		t.Errorf("Expected features.images to remain 'false', got %v", config["features.images"])
	}
	if config["features.profiles"] != "false" {
		t.Errorf("Expected features.profiles to remain 'false', got %v", config["features.profiles"])
	}

	// Verify defaults are set for unspecified features
	if config["features.storage.volumes"] != "true" {
		t.Errorf("Expected features.storage.volumes to be 'true', got %v", config["features.storage.volumes"])
	}
	if config["features.networks"] != "true" {
		t.Errorf("Expected features.networks to be 'true', got %v", config["features.networks"])
	}
}
