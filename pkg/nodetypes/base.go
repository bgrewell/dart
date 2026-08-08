package nodetypes

import (
	"encoding/json"
	"fmt"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/pkg/ifaces"
)

type BaseNode struct {
}

// knownNodeTypes mirrors the factory switch in CreateNodesWithWrappers;
// IsKnownNodeType lets validate-only paths (--check) reject unknown types
// without constructing real nodes.
var knownNodeTypes = map[string]bool{
	"local": true, "docker": true, "docker-compose": true,
	"ssh": true, "lxd": true, "lxd-vm": true,
}

// IsKnownNodeType reports whether the factory can construct this type.
func IsKnownNodeType(nodeType string) bool {
	return knownNodeTypes[nodeType]
}

// ValidateNodeOptions checks the option shapes that can be verified
// without contacting anything, so --check catches them before a run:
// unreadable known_hosts, missing SSH credentials, malformed bastion or
// volume specifications.
func ValidateNodeOptions(cfg *config.NodeConfig) error {
	switch cfg.Type {
	case "ssh":
		var opts SshNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		if _, err := sshAuthMethods(opts.KeyFile, opts.Pass); err != nil {
			return err
		}
		if _, err := hostKeyCallbackFor(opts.KnownHosts, opts.InsecureSkipHostKey); err != nil {
			return err
		}
		if opts.Bastion != nil {
			if opts.Bastion.Host == "" {
				return fmt.Errorf("bastion host is required")
			}
			if opts.Bastion.Bastion != nil {
				return fmt.Errorf("chained bastions are not supported: remove the nested bastion block")
			}
			if _, err := sshAuthMethods(opts.Bastion.KeyFile, opts.Bastion.Pass); err != nil {
				return fmt.Errorf("bastion: %w", err)
			}
		}
	case "docker", "docker-compose":
		var opts DockerNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		if _, err := resolveVolumes(opts.Volumes); err != nil {
			return err
		}
		if len(opts.Ports) > 0 {
			if err := docker.ValidatePortSpecs(opts.Ports); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeNodeOptions reuses the same JSON round-trip the factories use, so
// validation sees exactly what construction would.
func decodeNodeOptions(options map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(options)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// CreateNodes creates nodes using only the Docker wrapper (backward compatible)
func CreateNodes(configs []*config.NodeConfig, wrapper *docker.Wrapper) (map[string]ifaces.Node, error) {
	return CreateNodesWithWrappers(configs, wrapper, nil)
}

// CreateNodesWithWrappers creates nodes using both Docker and LXD wrappers
func CreateNodesWithWrappers(configs []*config.NodeConfig, dockerWrapper *docker.Wrapper, lxdWrapper *lxd.Wrapper) (map[string]ifaces.Node, error) {
	nodes := make(map[string]ifaces.Node)
	localNodeExists := false

	for _, cfg := range configs {
		if _, exists := nodes[cfg.Name]; exists {
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("duplicate node name %q", cfg.Name),
				Location: cfg.Loc,
			}
		}

		var node ifaces.Node
		var err error

		switch cfg.Type {
		case "local":
			if localNodeExists {
				return nil, &config.ConfigError{
					Message:  fmt.Sprintf("only one local node allowed; %q is a duplicate", cfg.Name),
					Location: cfg.Loc,
				}
			}
			node = NewLocalNode(cfg.Name, &cfg.Options)
			localNodeExists = true
		case "docker":
			node, err = NewDockerNode(dockerWrapper, cfg.Name, &cfg.Options)
		case "docker-compose":
			node, err = NewDockerComposeNode(dockerWrapper, cfg.Name, &cfg.Options)
		case "ssh":
			node, err = NewSshNode(cfg.Name, &cfg.Options)
		case "lxd":
			if lxdWrapper != nil {
				node, err = NewLxdNodeWithWrapper(lxdWrapper, cfg.Name, &cfg.Options)
			} else {
				node, err = NewLxdNode(cfg.Name, &cfg.Options)
			}
		case "lxd-vm":
			// Alias for LXD virtual machine type. The options map is copied
			// rather than mutated: cfg.Options belongs to the configuration.
			opts := make(map[string]interface{}, len(cfg.Options)+1)
			for k, v := range cfg.Options {
				opts[k] = v
			}
			opts["instance_type"] = "virtual-machine"
			if lxdWrapper != nil {
				node, err = NewLxdNodeWithWrapper(lxdWrapper, cfg.Name, &opts)
			} else {
				node, err = NewLxdNode(cfg.Name, &opts)
			}
		default:
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("unknown node type %q", cfg.Type),
				Location: cfg.TypeLoc,
			}
		}

		if err != nil {
			return nil, err
		}

		nodes[cfg.Name] = node
	}

	return nodes, nil
}
