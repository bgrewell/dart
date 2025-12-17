package steptypes

import (
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
)

// getTestNode returns a local node for testing with bash shell
func getTestNode() ifaces.Node {
	opts := map[string]interface{}{
		"shell": "/bin/bash",
	}
	return nodetypes.NewLocalNode(&opts)
}
