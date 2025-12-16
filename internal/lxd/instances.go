package lxd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ListInstances returns a list of instances that match the optional filter criteria
func ListInstances(ctx context.Context, server lxd.InstanceServer, filter *InstanceFilter) ([]api.Instance, error) {
	instances, err := server.GetInstances(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if filter == nil {
		return instances, nil
	}

	// Apply filters
	var filtered []api.Instance
	for _, inst := range instances {
		if filter.Name != "" && inst.Name != filter.Name {
			continue
		}
		if filter.Status != "" && inst.Status != filter.Status {
			continue
		}
		if filter.Type != "" && string(inst.Type) != filter.Type {
			continue
		}
		filtered = append(filtered, inst)
	}

	return filtered, nil
}

// GetInstance retrieves a specific instance by name
func GetInstance(ctx context.Context, server lxd.InstanceServer, name string) (*api.Instance, string, error) {
	instance, etag, err := server.GetInstance(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get instance %s: %w", name, err)
	}
	return instance, etag, nil
}

// CreateInstance creates a new instance (container or VM)
func CreateInstance(ctx context.Context, server lxd.InstanceServer, config *InstanceConfig) error {
	// Build the instance source
	source := api.InstanceSource{
		Type:     "image",
		Alias:    config.Image,
		Server:   config.ImageServer,
		Protocol: config.Protocol,
	}

	// Build device map
	devices := make(map[string]map[string]string)
	for name, dev := range config.Devices {
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

	// Create the instance request
	req := api.InstancesPost{
		Name:         config.Name,
		Type:         api.InstanceType(config.Type),
		Source:       source,
		InstanceType: config.Architecture,
		InstancePut: api.InstancePut{
			Profiles:  config.Profiles,
			Config:    config.Config,
			Devices:   devices,
			Ephemeral: config.Ephemeral,
		},
	}

	op, err := server.CreateInstance(req)
	if err != nil {
		return fmt.Errorf("failed to create instance %s: %w", config.Name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s creation: %w", config.Name, err)
	}

	return nil
}

// StartInstance starts an instance
func StartInstance(ctx context.Context, server lxd.InstanceServer, name string) error {
	req := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err := server.UpdateInstanceState(name, req, "")
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s to start: %w", name, err)
	}

	return nil
}

// StopInstance stops an instance
func StopInstance(ctx context.Context, server lxd.InstanceServer, name string, force bool) error {
	req := api.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
		Force:   force,
	}

	op, err := server.UpdateInstanceState(name, req, "")
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s to stop: %w", name, err)
	}

	return nil
}

// RestartInstance restarts an instance
func RestartInstance(ctx context.Context, server lxd.InstanceServer, name string, force bool) error {
	req := api.InstanceStatePut{
		Action:  "restart",
		Timeout: -1,
		Force:   force,
	}

	op, err := server.UpdateInstanceState(name, req, "")
	if err != nil {
		return fmt.Errorf("failed to restart instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s to restart: %w", name, err)
	}

	return nil
}

// DeleteInstance deletes an instance
func DeleteInstance(ctx context.Context, server lxd.InstanceServer, name string) error {
	op, err := server.DeleteInstance(name)
	if err != nil {
		return fmt.Errorf("failed to delete instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s deletion: %w", name, err)
	}

	return nil
}

// GetInstanceState returns the current state of an instance
func GetInstanceState(ctx context.Context, server lxd.InstanceServer, name string) (*api.InstanceState, string, error) {
	state, etag, err := server.GetInstanceState(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get instance %s state: %w", name, err)
	}
	return state, etag, nil
}

// ReadinessConfig holds configuration for waiting on instance readiness
type ReadinessConfig struct {
	// Timeout is the maximum time to wait for the instance to become ready
	Timeout time.Duration
	// PollInterval is how often to check the instance state
	PollInterval time.Duration
}

// DefaultReadinessConfig returns sensible defaults for readiness checking
func DefaultReadinessConfig() *ReadinessConfig {
	return &ReadinessConfig{
		Timeout:      5 * time.Minute,
		PollInterval: 2 * time.Second,
	}
}

// WaitForInstanceReady waits for an instance to be fully ready to accept commands.
// This checks that:
// 1. The instance state is "Running"
// 2. The instance has at least one network address (indicating networking is up)
// 3. A simple command can be executed successfully (indicating the OS is ready)
func WaitForInstanceReady(ctx context.Context, server lxd.InstanceServer, name string, config *ReadinessConfig) error {
	if config == nil {
		config = DefaultReadinessConfig()
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for instance %s to become ready: %w", name, ctx.Err())
		case <-ticker.C:
			ready, err := isInstanceReady(ctx, server, name)
			if err != nil {
				// Log but continue - the instance may still be initializing
				continue
			}
			if ready {
				return nil
			}
		}
	}
}

// isInstanceReady checks if an instance is fully ready to accept commands
func isInstanceReady(ctx context.Context, server lxd.InstanceServer, name string) (bool, error) {
	// Check instance state
	state, _, err := server.GetInstanceState(name)
	if err != nil {
		return false, fmt.Errorf("failed to get instance state: %w", err)
	}

	// Instance must be running
	if state.Status != "Running" {
		return false, nil
	}

	// Check if networking is available (at least one globally routable address)
	hasNetworkAddress := false
	for _, network := range state.Network {
		for _, addr := range network.Addresses {
			// Look for globally routable addresses (not loopback or link-local)
			if addr.Scope == "global" {
				hasNetworkAddress = true
				break
			}
		}
		if hasNetworkAddress {
			break
		}
	}

	if !hasNetworkAddress {
		return false, nil
	}

	// Try to execute a simple command to verify the instance is responsive
	var stdout, stderr bytes.Buffer
	exitCode, err := ExecInInstance(server, name, []string{"true"}, &stdout, &stderr)
	if err != nil {
		return false, nil // Instance not ready yet
	}

	return exitCode == 0, nil
}

// ExecInInstance runs a command in an instance and returns the exit code and output
func ExecInInstance(server lxd.InstanceServer, name string, command []string, stdout, stderr io.Writer) (int, error) {
	execArgs := lxd.InstanceExecArgs{
		Stdout: stdout,
		Stderr: stderr,
	}

	execPost := api.InstanceExecPost{
		Command:     command,
		WaitForWS:   true,
		Interactive: false,
	}

	op, err := server.ExecInstance(name, execPost, &execArgs)
	if err != nil {
		return -1, fmt.Errorf("failed to execute command in instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return -1, fmt.Errorf("failed waiting for command execution in instance %s: %w", name, err)
	}

	// Get exit code from operation metadata
	metadata := op.Get().Metadata
	exitCode, ok := metadata["return"].(float64)
	if !ok {
		return -1, fmt.Errorf("failed to get exit code from instance %s", name)
	}

	return int(exitCode), nil
}

// UpdateInstance updates an instance configuration
func UpdateInstance(ctx context.Context, server lxd.InstanceServer, name string, config map[string]string, etag string) error {
	instance, currentEtag, err := server.GetInstance(name)
	if err != nil {
		return fmt.Errorf("failed to get instance %s for update: %w", name, err)
	}

	if etag != "" {
		currentEtag = etag
	}

	// Merge config
	for k, v := range config {
		instance.Config[k] = v
	}

	// Update instance writable fields
	writable := instance.Writable()

	op, err := server.UpdateInstance(name, writable, currentEtag)
	if err != nil {
		return fmt.Errorf("failed to update instance %s: %w", name, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for instance %s update: %w", name, err)
	}

	return nil
}

// GetInstanceSnapshots returns all snapshots for an instance
func GetInstanceSnapshots(ctx context.Context, server lxd.InstanceServer, name string) ([]api.InstanceSnapshot, error) {
	snapshots, err := server.GetInstanceSnapshots(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots for instance %s: %w", name, err)
	}
	return snapshots, nil
}

// CreateInstanceSnapshot creates a snapshot of an instance
func CreateInstanceSnapshot(ctx context.Context, server lxd.InstanceServer, instanceName, snapshotName string, stateful bool) error {
	req := api.InstanceSnapshotsPost{
		Name:     snapshotName,
		Stateful: stateful,
	}

	op, err := server.CreateInstanceSnapshot(instanceName, req)
	if err != nil {
		return fmt.Errorf("failed to create snapshot %s for instance %s: %w", snapshotName, instanceName, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for snapshot %s creation: %w", snapshotName, err)
	}

	return nil
}

// DeleteInstanceSnapshot deletes a snapshot
func DeleteInstanceSnapshot(ctx context.Context, server lxd.InstanceServer, instanceName, snapshotName string) error {
	op, err := server.DeleteInstanceSnapshot(instanceName, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot %s for instance %s: %w", snapshotName, instanceName, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for snapshot %s deletion: %w", snapshotName, err)
	}

	return nil
}
