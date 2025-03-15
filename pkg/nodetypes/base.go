package nodetypes

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

type BaseNode struct {
}

func CreateNodes(configs []*config.NodeConfig, wrapper *docker.Wrapper) (map[string]ifaces.Node, error) {
	nodes := make(map[string]ifaces.Node)
	localNodeExists := false

	for _, cfg := range configs {
		if _, exists := nodes[cfg.Name]; exists {
			return nil, helpers.ErrNodeAlreadyExists
		}

		var node ifaces.Node
		var err error

		switch cfg.Type {
		case "local":
			if localNodeExists {
				return nil, helpers.ErrLocalNodeAlreadyExists
			}
			node = NewLocalNode(&cfg.Options)
			localNodeExists = true
		case "docker":
			node, err = NewDockerNode(wrapper, cfg.Name, &cfg.Options)
		case "ssh":
			node, err = NewSshNode(&cfg.Options)
		case "lxd":
			node, err = NewLxdNode(cfg.Name, &cfg.Options)
		default:
			return nil, helpers.ErrUnknownNodeType
		}

		if err != nil {
			return nil, err
		}

		nodes[cfg.Name] = node
	}

	return nodes, nil
}
