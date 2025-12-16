package lxd

import (
	"context"
	"fmt"
	"net"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ListNetworks returns a list of all networks
func ListNetworks(ctx context.Context, server lxd.InstanceServer) ([]api.Network, error) {
	networks, err := server.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	return networks, nil
}

// GetNetwork retrieves a specific network by name
func GetNetwork(ctx context.Context, server lxd.InstanceServer, name string) (*api.Network, string, error) {
	network, etag, err := server.GetNetwork(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get network %s: %w", name, err)
	}
	return network, etag, nil
}

// CreateNetwork creates a new network
func CreateNetwork(ctx context.Context, server lxd.InstanceServer, name, networkType string, config map[string]string) error {
	if config == nil {
		config = make(map[string]string)
	}

	req := api.NetworksPost{
		Name: name,
		Type: networkType,
		NetworkPut: api.NetworkPut{
			Config: config,
		},
	}

	err := server.CreateNetwork(req)
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", name, err)
	}

	return nil
}

// CreateBridgeNetwork creates a bridge network with specific subnet and gateway
func CreateBridgeNetwork(ctx context.Context, server lxd.InstanceServer, name, subnet, gateway string) error {
	config := map[string]string{
		"ipv4.address": gateway + "/" + getSubnetMask(subnet),
		"ipv4.nat":     "true",
	}

	return CreateNetwork(ctx, server, name, "bridge", config)
}

// UpdateNetwork updates an existing network configuration
func UpdateNetwork(ctx context.Context, server lxd.InstanceServer, name string, config map[string]string, etag string) error {
	network, currentEtag, err := server.GetNetwork(name)
	if err != nil {
		return fmt.Errorf("failed to get network %s for update: %w", name, err)
	}

	if etag != "" {
		currentEtag = etag
	}

	// Merge config
	for k, v := range config {
		network.Config[k] = v
	}

	err = server.UpdateNetwork(name, network.Writable(), currentEtag)
	if err != nil {
		return fmt.Errorf("failed to update network %s: %w", name, err)
	}

	return nil
}

// DeleteNetwork deletes a network
func DeleteNetwork(ctx context.Context, server lxd.InstanceServer, name string) error {
	err := server.DeleteNetwork(name)
	if err != nil {
		return fmt.Errorf("failed to delete network %s: %w", name, err)
	}
	return nil
}

// GetNetworkState returns the current state of a network
func GetNetworkState(ctx context.Context, server lxd.InstanceServer, name string) (*api.NetworkState, error) {
	state, err := server.GetNetworkState(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get network %s state: %w", name, err)
	}
	return state, nil
}

// AttachNetworkToInstance attaches a network device to an instance
func AttachNetworkToInstance(ctx context.Context, server lxd.InstanceServer, instanceName, networkName, deviceName string) error {
	return AttachNetworkToInstanceWithIP(ctx, server, instanceName, networkName, deviceName, "")
}

// AttachNetworkToInstanceWithIP attaches a network device to an instance with an optional static IP address
func AttachNetworkToInstanceWithIP(ctx context.Context, server lxd.InstanceServer, instanceName, networkName, deviceName, ipAddress string) error {
	instance, etag, err := server.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance %s: %w", instanceName, err)
	}

	if instance.Devices == nil {
		instance.Devices = make(map[string]map[string]string)
	}

	deviceConfig := map[string]string{
		"type":    "nic",
		"network": networkName,
	}

	// If a static IP address is specified, detect type and add it to the device configuration
	if ipAddress != "" {
		ip := net.ParseIP(ipAddress)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", ipAddress)
		}
		if ip.To4() != nil {
			deviceConfig["ipv4.address"] = ipAddress
		} else {
			deviceConfig["ipv6.address"] = ipAddress
		}
	}

	instance.Devices[deviceName] = deviceConfig

	op, err := server.UpdateInstance(instanceName, instance.Writable(), etag)
	if err != nil {
		return fmt.Errorf("failed to attach network %s to instance %s: %w", networkName, instanceName, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for network attachment: %w", err)
	}

	return nil
}

// DetachNetworkFromInstance removes a network device from an instance
func DetachNetworkFromInstance(ctx context.Context, server lxd.InstanceServer, instanceName, deviceName string) error {
	instance, etag, err := server.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance %s: %w", instanceName, err)
	}

	if _, exists := instance.Devices[deviceName]; !exists {
		return fmt.Errorf("device %s not found on instance %s", deviceName, instanceName)
	}

	delete(instance.Devices, deviceName)

	op, err := server.UpdateInstance(instanceName, instance.Writable(), etag)
	if err != nil {
		return fmt.Errorf("failed to detach network device %s from instance %s: %w", deviceName, instanceName, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for network detachment: %w", err)
	}

	return nil
}

// getSubnetMask extracts the CIDR mask from a subnet string (e.g., "10.0.0.0/24" -> "24")
func getSubnetMask(subnet string) string {
	for i := len(subnet) - 1; i >= 0; i-- {
		if subnet[i] == '/' {
			return subnet[i+1:]
		}
	}
	return "24" // default to /24
}
