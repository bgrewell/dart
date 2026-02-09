package nodetypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/pkg/ifaces"
)

type BaseNode struct {
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
			node = NewLocalNode(&cfg.Options)
			localNodeExists = true
		case "docker":
			node, err = NewDockerNode(dockerWrapper, cfg.Name, &cfg.Options)
		case "docker-compose":
			node, err = NewDockerComposeNode(dockerWrapper, cfg.Name, &cfg.Options)
		case "ssh":
			node, err = NewSshNode(&cfg.Options)
		case "lxd":
			if lxdWrapper != nil {
				node, err = NewLxdNodeWithWrapper(lxdWrapper, cfg.Name, &cfg.Options)
			} else {
				node, err = NewLxdNode(cfg.Name, &cfg.Options)
			}
		case "lxd-vm":
			// Alias for LXD virtual machine type
			opts := cfg.Options
			if opts == nil {
				opts = make(map[string]interface{})
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
