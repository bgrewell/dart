package nodetypes

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Node = &DockerNode{}

type DockerNetworkOpts struct {
	Name   string `yaml:"name,omitempty" json:"name"`
	Subnet string `yaml:"subnet,omitempty" json:"subnet"`
	Ip     string `yaml:"ip,omitempty" json:"ip"`
}

type DockerNodeOpts struct {
	Image string `yaml:"image,omitempty" json:"image"`
	// ContainerName decouples the Docker object's name from the node name.
	// Defaults to the node name, which is what makes a suite's containers
	// findable by the name the YAML uses.
	ContainerName string `yaml:"container_name,omitempty" json:"container_name"`
	// Command and Entrypoint override the image's CMD and ENTRYPOINT. An
	// image whose default command exits immediately cannot host a node
	// otherwise.
	Command     []string               `yaml:"command,omitempty" json:"command"`
	Entrypoint  []string               `yaml:"entrypoint,omitempty" json:"entrypoint"`
	ExecOptions map[string]interface{} `yaml:"exec_opts,omitempty" json:"exec_opts"`
	Networks    []DockerNetworkOpts    `yaml:"networks,omitempty" json:"networks"`
	// Privileged grants the container full host capabilities. It is
	// opt-in: DART used to set it unconditionally, which quietly gave
	// every test container far more access than a test needs.
	Privileged bool     `yaml:"privileged,omitempty" json:"privileged"`
	Volumes    []string `yaml:"volumes,omitempty" json:"volumes"` // host:container[:ro]
	Env        []string `yaml:"env,omitempty" json:"env"`         // KEY=VALUE
	Ports      []string `yaml:"ports,omitempty" json:"ports"`     // host:container[/proto]
	// Capabilities adds specific Linux capabilities without going
	// privileged (e.g. NET_ADMIN for network tests).
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities"`
}

func NewDockerNode(wrapper *docker.Wrapper, name string, opts ifaces.NodeOptions, suiteDir string) (node ifaces.Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts DockerNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	return &DockerNode{
		name:     name,
		wrapper:  wrapper,
		options:  nodeopts,
		suiteDir: suiteDir,
	}, nil
}

type DockerNode struct {
	name     string
	wrapper  *docker.Wrapper
	options  DockerNodeOpts
	suiteDir string
}

// resolveVolumes turns relative host paths into absolute ones. The Engine
// API treats a non-absolute source as a NAMED VOLUME, so "./fixtures"
// would silently mount an empty volume instead of the directory —
// a passing-looking test against missing data.
func resolveVolumes(volumes []string, suiteDir string) ([]string, error) {
	resolved := make([]string, 0, len(volumes))
	for _, spec := range volumes {
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("volume %q must be host:container[:options]", spec)
		}
		source := parts[0]
		// A bare name (no separator) is a named volume by intent; a path
		// is anything containing a separator or starting with . or ~
		if strings.HasPrefix(source, "~") || (strings.ContainsAny(source, "/.") && !filepath.IsAbs(source)) {
			resolved, err := config.ResolveLocalPath(suiteDir, source)
			if err != nil {
				return nil, fmt.Errorf("volume %q: %w", spec, err)
			}
			source = resolved
		}
		resolved = append(resolved, source+":"+parts[1])
	}
	return resolved, nil
}

func (d *DockerNode) Setup() error {
	// Fetch the image first: creating a container from an absent image fails
	// with the daemon's "No such image", which describes the symptom rather
	// than the cause
	if err := d.wrapper.EnsureImage(d.options.Image); err != nil {
		return err
	}

	var opts []docker.ContainerOptions
	if d.options.Privileged {
		opts = append(opts, docker.WithPrivileged())
	}
	if len(d.options.Capabilities) > 0 {
		opts = append(opts, docker.WithCapabilities(d.options.Capabilities))
	}
	if len(d.options.Volumes) > 0 {
		volumes, err := resolveVolumes(d.options.Volumes, d.suiteDir)
		if err != nil {
			return err
		}
		opts = append(opts, docker.WithVolumes(volumes))
	}
	if len(d.options.Env) > 0 {
		opts = append(opts, docker.WithEnv(d.options.Env))
	}
	if len(d.options.Ports) > 0 {
		opts = append(opts, docker.WithPorts(d.options.Ports))
	}
	if len(d.options.Command) > 0 {
		opts = append(opts, docker.WithCommand(d.options.Command))
	}
	if len(d.options.Entrypoint) > 0 {
		opts = append(opts, docker.WithEntrypoint(d.options.Entrypoint))
	}
	if len(d.options.Networks) > 0 {
		attachments := make([]docker.NetworkAttachment, 0, len(d.options.Networks))
		for _, net := range d.options.Networks {
			attachments = append(attachments, docker.NetworkAttachment{Name: net.Name, IPv4: net.Ip})
		}
		opts = append(opts, docker.WithNetworks(attachments))
	}

	// The hostname stays the node name even when the container is named
	// something else, so node-side commands see the name the suite uses
	if err := d.wrapper.CreateContainer(d.containerName(), d.name, d.options.Image, opts...); err != nil {
		return err
	}
	if err := d.wrapper.StartContainer(d.containerName()); err != nil {
		return err
	}
	// Wait for the container to be fully ready (running and responsive)
	if err := d.wrapper.WaitForContainerReady(d.containerName()); err != nil {
		return err
	}
	return nil
}

// containerName is the Docker object's name. It defaults to the node name,
// so a suite's containers are findable by the name the YAML uses;
// container_name overrides it for suites that must match an externally
// fixed name.
func (d *DockerNode) containerName() string {
	if d.options.ContainerName != "" {
		return d.options.ContainerName
	}
	return d.name
}

// Teardown stops and removes the container. A container that no longer
// exists (partial setup, previous cleanup, teardown-only run) counts as
// already removed.
func (d *DockerNode) Teardown() error {
	if err := d.wrapper.StopContainer(d.containerName()); err != nil {
		if docker.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := d.wrapper.RemoveContainer(d.containerName()); err != nil && !docker.IsNotFound(err) {
		return err
	}
	return nil
}

func (d *DockerNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	code, stdout, stderr, err := d.wrapper.ExecuteInContainerStreaming(d.containerName(), command, execution.IsDebugMode())
	if err != nil {
		return nil, err
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    code,
		Stdout:      stdout,
		Stderr:      stderr,
	}, nil
}

var _ ifaces.NetworkInspector = &DockerNode{}

// NetworkFacts reports the container's addresses from Docker's own
// inspection data, so suites can reference {{ fact "node" "ipv4" }}
// without a fact command. Each attached network also yields a
// per-network fact ("ipv4.test-net").
func (d *DockerNode) NetworkFacts() (map[string]string, error) {
	return d.wrapper.ContainerNetworkFacts(d.containerName())
}

// Close has nothing to release: the container lifecycle is handled by
// Setup/Teardown and the client belongs to the shared wrapper. Returning
// an error here would fail every run's final cleanup.
func (d *DockerNode) Close() error {
	return nil
}
