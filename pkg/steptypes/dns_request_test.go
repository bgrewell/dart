package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDNSRequestStep verifies resolution from both vantages. localhost is
// in every hosts file, so the answer is stable without a network.
func TestDNSRequestStep(t *testing.T) {
	for _, from := range []string{"node", "host"} {
		t.Run("from="+from, func(t *testing.T) {
			node := ifaces.Node(nodetypes.NewMockNode())
			if from == "node" {
				node = localNode(t)
			}

			step, err := makeStepOn(t, node, TypeDNSRequest, map[string]interface{}{
				"hostname":     "localhost",
				"expected_ips": []interface{}{"127.0.0.1"},
				"from":         from,
			})
			require.NoError(t, err)
			require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

			// An address the name does not carry fails
			wrong, err := makeStepOn(t, node, TypeDNSRequest, map[string]interface{}{
				"hostname":     "localhost",
				"expected_ips": []interface{}{"203.0.113.7"},
				"from":         from,
			})
			require.NoError(t, err)
			assert.ErrorContains(t, wrong.Run(formatters.NewMockTaskCompleter()), "expected IP 203.0.113.7 not found")

			// A name that cannot resolve is an error, not an empty pass
			missing, err := makeStepOn(t, node, TypeDNSRequest, map[string]interface{}{
				"hostname": "dart-nonexistent.invalid",
				"timeout":  5,
				"from":     from,
			})
			require.NoError(t, err)
			assert.ErrorContains(t, missing.Run(formatters.NewMockTaskCompleter()), "DNS resolution failed")
		})
	}
}
