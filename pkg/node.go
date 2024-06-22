package pkg

import (
	"github.com/bgrewell/dart/internal"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/execution"
)

// Node is an interface representing a computing entity (e.g., a server, VM, or container)
// that can be used as a target for test operations, such as executing commands or participating
// in distributed systems for testing purposes.
type Node interface {
	Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error)
	Close() error
}

func CreateNodes(configs []config.NodeConfig) (nodes map[string]Node, err error) {
	nodes = make(map[string]Node)
	localNodeExists := false
	for _, cfg := range configs {
		switch cfg.Type {
		case "local":
			if localNodeExists {
				return nil, internal.ErrLocalNodeAlreadyExists
			}
			if _, ok := nodes[cfg.Name]; ok {
				return nil, internal.ErrNodeAlreadyExists
			}

			var options []execution.ExecutionOption
			if cfg.Options != nil {
				options = execution.OptionsToExecutionOptions(cfg.Options)
			}

			nodes[cfg.Name] = NewLocalNode(options...)
		}
	}
	return
}
