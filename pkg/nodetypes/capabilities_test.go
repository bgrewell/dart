package nodetypes

import (
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capabilitiesOf reports which capability interfaces a node satisfies.
func capabilitiesOf(node interface{}) map[Capability]bool {
	found := map[Capability]bool{}
	if _, ok := node.(ifaces.Rebooter); ok {
		found[CapabilityReboot] = true
	}
	if _, ok := node.(ifaces.Snapshotter); ok {
		found[CapabilitySnapshot] = true
	}
	if _, ok := node.(ifaces.NetworkInspector); ok {
		found[CapabilityNetworkInspector] = true
	}
	return found
}

// The table must match what the node types actually implement. A node type
// that gains or loses a capability without the table following would put
// --check back out of step with a real run — the defect this replaced.
//
// Zero-value pointers satisfy interfaces without construction, so no daemon
// or connection is needed here.
func TestCapabilityTableMatchesNodeTypes(t *testing.T) {
	real := map[string]interface{}{
		"local":          &LocalNode{},
		"docker":         &DockerNode{},
		"docker-compose": &DockerComposeNode{},
		"ssh":            &SshNode{},
		"lxd":            &LxdNode{},
		"lxd-vm":         &LxdNode{},
	}

	for nodeType, node := range real {
		actual := capabilitiesOf(node)
		for _, capability := range []Capability{CapabilityReboot, CapabilitySnapshot, CapabilityNetworkInspector} {
			assert.Equal(t, actual[capability], Supports(nodeType, capability),
				"table and implementation disagree: %s / %s", nodeType, capability)
		}
	}

	// Every type the factory accepts must appear above, or a new node type
	// could ship with its capabilities unrecorded
	for nodeType := range knownNodeTypes {
		_, covered := real[nodeType]
		assert.True(t, covered, "node type %q is missing from the capability table test", nodeType)
	}
}

// A stand-in must implement exactly the capabilities of the type it stands
// for. More would pass a suite the run rejects; fewer would reject a suite
// the run accepts.
func TestCheckNodeMirrorsRealCapabilities(t *testing.T) {
	for nodeType := range knownNodeTypes {
		stand := NewCheckNode(nodeType)
		actual := capabilitiesOf(stand)

		for _, capability := range []Capability{CapabilityReboot, CapabilitySnapshot} {
			assert.Equal(t, Supports(nodeType, capability), actual[capability],
				"stand-in for %s: %s", nodeType, capability)
		}
	}
}

// --check validates configuration, not behaviour, so the stand-in accepts
// any command rather than requiring a registered response.
func TestCheckNodeAcceptsAnyCommand(t *testing.T) {
	result, err := NewCheckNode("local").Execute("anything at all")
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
}

func TestSupportingTypesIsSortedAndComplete(t *testing.T) {
	assert.Equal(t, "lxd, lxd-vm, ssh", SupportingTypes(CapabilityReboot))
	assert.Equal(t, "lxd, lxd-vm", SupportingTypes(CapabilitySnapshot))
}
