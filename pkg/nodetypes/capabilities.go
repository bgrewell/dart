package nodetypes

import (
	"sort"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// Capability names an optional behaviour a node type may support beyond
// running commands. Steps and tests that need one assert for the matching
// interface at construction.
type Capability string

const (
	CapabilityReboot           Capability = "reboot"
	CapabilitySnapshot         Capability = "snapshot"
	CapabilityNetworkInspector Capability = "network inspection"
)

// nodeCapabilities records which node types implement which capability.
// It is the single source of truth for both the real construction-time
// checks and the stand-in nodes --check builds, so validation cannot
// disagree with a run. TestCapabilityTableMatchesNodeTypes guards it
// against drift.
var nodeCapabilities = map[Capability]map[string]bool{
	CapabilityReboot: {
		"ssh": true, "lxd": true, "lxd-vm": true,
	},
	CapabilitySnapshot: {
		"lxd": true, "lxd-vm": true,
	},
	CapabilityNetworkInspector: {
		"docker": true, "lxd": true, "lxd-vm": true,
	},
}

// Supports reports whether a node type implements a capability.
func Supports(nodeType string, capability Capability) bool {
	return nodeCapabilities[capability][nodeType]
}

// SupportingTypes lists the node types implementing a capability, for the
// "supported: ..." half of an error message.
func SupportingTypes(capability Capability) string {
	types := make([]string, 0, len(nodeCapabilities[capability]))
	for nodeType := range nodeCapabilities[capability] {
		types = append(types, nodeType)
	}
	sort.Strings(types)
	return strings.Join(types, ", ")
}

// NewCheckNode returns a stand-in node for --check that implements exactly
// the capabilities the named type really has. A stand-in that implements
// more would pass a suite the run rejects; one that implements fewer would
// reject a suite the run accepts. Both happened.
func NewCheckNode(nodeType string) ifaces.Node {
	base := checkNode{MockNode: NewMockNode()}

	switch {
	case Supports(nodeType, CapabilityReboot) && Supports(nodeType, CapabilitySnapshot):
		return &checkNodeRebootSnapshot{checkNodeReboot: checkNodeReboot{checkNode: base}}
	case Supports(nodeType, CapabilityReboot):
		return &checkNodeReboot{checkNode: base}
	default:
		return &base
	}
}

// checkNode answers every command successfully: --check validates
// configuration, never behaviour.
type checkNode struct {
	*MockNode
}

// Execute accepts any command, unlike MockNode which requires a registered
// response — a step's command text is not known at check time.
func (c *checkNode) Execute(command string, options ...execution.ExecutionOption) (*execution.ExecutionResult, error) {
	return &execution.ExecutionResult{
		ExitCode: 0,
		Stdout:   strings.NewReader(""),
		Stderr:   strings.NewReader(""),
	}, nil
}

type checkNodeReboot struct {
	checkNode
}

func (c *checkNodeReboot) Reboot(force bool, readyCommand string, timeout time.Duration) error {
	return nil
}

type checkNodeRebootSnapshot struct {
	checkNodeReboot
}

func (c *checkNodeRebootSnapshot) Snapshot(name string, stateful bool) error { return nil }

func (c *checkNodeRebootSnapshot) RestoreSnapshot(name string, stateful bool) error { return nil }

func (c *checkNodeRebootSnapshot) DeleteSnapshot(name string) error { return nil }
