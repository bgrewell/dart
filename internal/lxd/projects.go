package lxd

import (
	"context"
	"fmt"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ListProjects returns a list of all projects
func ListProjects(ctx context.Context, server lxd.InstanceServer) ([]api.Project, error) {
	projects, err := server.GetProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// GetProject retrieves a specific project by name
func GetProject(ctx context.Context, server lxd.InstanceServer, name string) (*api.Project, string, error) {
	project, etag, err := server.GetProject(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project %s: %w", name, err)
	}
	return project, etag, nil
}

// CreateProject creates a new project with the given name and configuration
func CreateProject(ctx context.Context, server lxd.InstanceServer, name string, config map[string]string, description string) error {
	if config == nil {
		config = make(map[string]string)
	}

	// Set default project features if not specified
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

	req := api.ProjectsPost{
		Name: name,
		ProjectPut: api.ProjectPut{
			Description: description,
			Config:      config,
		},
	}

	err := server.CreateProject(req)
	if err != nil {
		return fmt.Errorf("failed to create project %s: %w", name, err)
	}

	return nil
}

// UpdateProject updates an existing project configuration
func UpdateProject(ctx context.Context, server lxd.InstanceServer, name string, config map[string]string, description string, etag string) error {
	project, currentEtag, err := server.GetProject(name)
	if err != nil {
		return fmt.Errorf("failed to get project %s for update: %w", name, err)
	}

	if etag != "" {
		currentEtag = etag
	}

	// Merge config
	for k, v := range config {
		project.Config[k] = v
	}

	if description != "" {
		project.Description = description
	}

	err = server.UpdateProject(name, project.Writable(), currentEtag)
	if err != nil {
		return fmt.Errorf("failed to update project %s: %w", name, err)
	}

	return nil
}

// DeleteProject deletes a project
func DeleteProject(ctx context.Context, server lxd.InstanceServer, name string) error {
	err := server.DeleteProject(name)
	if err != nil {
		return fmt.Errorf("failed to delete project %s: %w", name, err)
	}
	return nil
}

// CopyProfileToProject copies a profile from one project to another
// This is useful for copying the default profile when creating new projects
func CopyProfileToProject(ctx context.Context, server lxd.InstanceServer, sourceProject, targetProject, profileName string) error {
	// Use the target project context to check if profile already exists
	targetServer := server.UseProject(targetProject)
	_, _, err := targetServer.GetProfile(profileName)
	if err == nil {
		// Profile already exists, no need to copy
		return nil
	}

	// Use the source project context to get the profile
	sourceServer := server.UseProject(sourceProject)
	profile, _, err := sourceServer.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get profile %s from project %s: %w", profileName, sourceProject, err)
	}

	// Create the profile in the target project
	req := api.ProfilesPost{
		Name: profileName,
		ProfilePut: api.ProfilePut{
			Description: profile.Description,
			Config:      profile.Config,
			Devices:     profile.Devices,
		},
	}

	err = targetServer.CreateProfile(req)
	if err != nil {
		return fmt.Errorf("failed to create profile %s in project %s: %w", profileName, targetProject, err)
	}

	return nil
}

// EnsureDefaultProfile ensures that the default profile exists in a project
// If it doesn't exist, it copies it from the default project
func EnsureDefaultProfile(ctx context.Context, server lxd.InstanceServer, projectName string) error {
	projectServer := server.UseProject(projectName)
	
	// Check if default profile exists
	_, _, err := projectServer.GetProfile("default")
	if err == nil {
		// Profile already exists
		return nil
	}

	// Profile doesn't exist, copy it from the default project
	return CopyProfileToProject(ctx, server, "default", projectName, "default")
}
