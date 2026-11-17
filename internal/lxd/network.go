package lxd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

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

	return waitForNetwork(ctx, server, name, true)
}

// Networks are polled this often, for at most this long, after a create or
// delete request. Variables rather than constants so tests can shorten them.
var (
	networkPollInterval = 200 * time.Millisecond
	networkWaitTimeout  = 60 * time.Second
)

// waitForNetwork blocks until the named network is fully created
// (present=true) or no longer exists (present=false).
//
// Newer LXD servers create and delete networks asynchronously: the request is
// answered with a background operation, which the client's CreateNetwork and
// DeleteNetwork neither return nor wait for. Without this wait, an instance
// created straight after its network fails with "Network not found", and a
// suite can exit while its networks are still being created or removed.
func waitForNetwork(ctx context.Context, server lxd.InstanceServer, name string, present bool) error {
	want := "created"
	if !present {
		want = "removed"
	}

	deadline := time.Now().Add(networkWaitTimeout)
	for {
		network, _, err := server.GetNetwork(name)
		switch {
		case err != nil && !IsNotFound(err):
			return fmt.Errorf("failed to check network %s while waiting for it to be %s: %w", name, want, err)
		case err != nil && !present:
			return nil
		case err == nil && present && network.Status == api.NetworkStatusCreated:
			return nil
		case err == nil && present && network.Status == api.NetworkStatusErrored:
			return fmt.Errorf("network %s was created in the %s state", name, network.Status)
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for network %s to be %s", networkWaitTimeout, name, want)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("stopped waiting for network %s to be %s: %w", name, want, ctx.Err())
		case <-time.After(networkPollInterval):
		}
	}
}

// CreateBridgeNetwork creates a bridge network with specific subnet and gateway.
// nat controls NAT for both address families; disabling it yields an isolated
// (air-gapped) bridge where instances get DHCP addresses but no route to the
// outside world. ipv6.nat must be set explicitly — LXD auto-assigns an IPv6
// subnet with NAT enabled by default, which would leak internet access.
// The subnet and gateway are validated here so a malformed config fails with
// a clear message instead of an obscure server-side error.
func CreateBridgeNetwork(ctx context.Context, server lxd.InstanceServer, name, subnet, gateway string, nat bool) error {
	if _, _, err := net.ParseCIDR(subnet); err != nil {
		return fmt.Errorf("network %s: subnet %q is not valid CIDR notation: %w", name, subnet, err)
	}
	if net.ParseIP(gateway) == nil {
		return fmt.Errorf("network %s: gateway %q is not a valid IP address", name, gateway)
	}

	config := map[string]string{
		"ipv4.address": gateway + "/" + getSubnetMask(subnet),
		"ipv4.nat":     strconv.FormatBool(nat),
		"ipv6.nat":     strconv.FormatBool(nat),
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
	return waitForNetwork(ctx, server, name, false)
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
