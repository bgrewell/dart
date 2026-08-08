package testtypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/probe"
)

// Vantage aliases the shared type so test option parsing reads naturally.
type Vantage = probe.Vantage

const (
	VantageNode = probe.VantageNode
	VantageHost = probe.VantageHost
)

// parseVantage reads the `from` option, defaulting to the test's node.
func parseVantage(testName string, opts map[string]interface{}) (Vantage, error) {
	value, _, err := optString(testName, opts, "from")
	if err != nil {
		return "", err
	}
	vantage, ok := probe.ParseVantage(value)
	if !ok {
		return "", fmt.Errorf("from must be %q or %q in test %q (got %q)",
			probe.VantageNode, probe.VantageHost, testName, value)
	}
	return vantage, nil
}

// requireTool builds a shell guard that fails loudly when a probe's
// dependency is missing on the node.
func requireTool(tool string) string {
	return probe.RequireTool(tool)
}
