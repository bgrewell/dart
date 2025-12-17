package lxd

// InstanceFilter holds criteria for filtering instances
type InstanceFilter struct {
	Name   string
	Status string
	Type   string // "container" or "virtual-machine"
}

// Network represents LXD network configuration
type Network struct {
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"` // "bridge", "ovn", etc.
	Subnet  string `json:"subnet" yaml:"subnet"`
	Gateway string `json:"gateway" yaml:"gateway"`
}

// Image represents LXD image configuration
type Image struct {
	Alias    string `json:"alias" yaml:"alias"`
	Server   string `json:"server" yaml:"server"`
	Protocol string `json:"protocol" yaml:"protocol"` // "lxd" or "simplestreams"
}

// Profile represents LXD profile configuration
type Profile struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Config      map[string]string `json:"config" yaml:"config"`
	Devices     map[string]Device `json:"devices" yaml:"devices"`
}

// Device represents a device configuration for profiles/instances
type Device struct {
	Type string            `json:"type" yaml:"type"` // "disk", "nic", "unix-char", etc.
	Path string            `json:"path,omitempty" yaml:"path,omitempty"`
	Pool string            `json:"pool,omitempty" yaml:"pool,omitempty"`
	Name string            `json:"name,omitempty" yaml:"name,omitempty"`
	Opts map[string]string `json:"opts,omitempty" yaml:"opts,omitempty"`
}

// InstanceType defines whether an instance is a container or VM
type InstanceType string

const (
	// InstanceTypeContainer represents an LXC container
	InstanceTypeContainer InstanceType = "container"
	// InstanceTypeVM represents a virtual machine
	InstanceTypeVM InstanceType = "virtual-machine"
)

// InstanceConfig holds configuration for creating an instance
type InstanceConfig struct {
	Name         string            `json:"name" yaml:"name"`
	Type         InstanceType      `json:"type" yaml:"type"`
	Image        string            `json:"image" yaml:"image"`
	ImageServer  string            `json:"image_server" yaml:"image_server"`
	Protocol     string            `json:"protocol" yaml:"protocol"`
	Profiles     []string          `json:"profiles" yaml:"profiles"`
	Config       map[string]string `json:"config" yaml:"config"`
	Devices      map[string]Device `json:"devices" yaml:"devices"`
	Ephemeral    bool              `json:"ephemeral" yaml:"ephemeral"`
	Architecture string            `json:"architecture" yaml:"architecture"`
}

// ProjectConfig holds configuration for an LXD project
type ProjectConfig struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Config      map[string]string `json:"config" yaml:"config"`
}
