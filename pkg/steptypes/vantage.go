package steptypes

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/probe"
)

// Vantage aliases the shared type so step option parsing reads naturally.
type Vantage = probe.Vantage

const (
	VantageNode = probe.VantageNode
	VantageHost = probe.VantageHost
)

// parseVantage reads the `from` option, defaulting to the step's node.
func parseVantage(c *config.StepConfig) (Vantage, error) {
	value, _, err := optString(c, "from")
	if err != nil {
		return "", err
	}
	vantage, ok := probe.ParseVantage(value)
	if !ok {
		return "", optionError(c, "from must be %q or %q in step %q (got %q)",
			probe.VantageNode, probe.VantageHost, c.Name, value)
	}
	return vantage, nil
}
