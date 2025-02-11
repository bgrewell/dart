package internal

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/execution"
)

type NodeOptions *map[string]interface{}

// Node is an interface representing a computing entity (e.g., a server, VM, or container)
// that can be used as a target for test operations, such as executing commands or participating
// in distributed systems for testing purposes.
type Node interface {
	Setup() error
	Teardown() error
	Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error)
	Close() error
}

func CreateNodes(configs []*config.NodeConfig, wrapper *docker.Wrapper) (nodes map[string]Node, err error) {
	nodes = make(map[string]Node)
	localNodeExists := false
	for _, cfg := range configs {
		if _, ok := nodes[cfg.Name]; ok {
			return nil, ErrNodeAlreadyExists
		}
		switch cfg.Type {
		case "local":
			if localNodeExists {
				return nil, ErrLocalNodeAlreadyExists
			}

			nodes[cfg.Name] = NewLocalNode(&cfg.Options)
		case "docker":
			node, err := NewDockerNode(wrapper, cfg.Name, &cfg.Options)
			if err != nil {
				return nil, err
			}
			nodes[cfg.Name] = node
		case "ssh":
			node, err := NewSshNode(&cfg.Options)
			if err != nil {
				return nil, err
			}
			nodes[cfg.Name] = node
		case "lxd":
			node, err := NewLxdNode(cfg.Name, &cfg.Options)
			if err != nil {
				return nil, err
			}
			nodes[cfg.Name] = node
		default:
			return nil, ErrUnknownNodeType
		}
	}
	return
}
