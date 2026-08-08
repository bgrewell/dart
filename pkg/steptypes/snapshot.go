package steptypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
)

var _ ifaces.Step = &SnapshotStep{}

// SnapshotStep captures, restores, or deletes a snapshot of the target
// node, giving destructive tests cheap isolation: snapshot in setup,
// break things, restore in teardown — far faster than recreating a node.
// Supported on node types implementing ifaces.Snapshotter (lxd).
type SnapshotStep struct {
	BaseStep
	node     ifaces.Node
	action   string
	name     string
	stateful bool
}

// newSnapshotStep parses name (required) and action
// (create|restore|delete, default create); stateful includes running
// memory and requires host support (CRIU). A stateful snapshot must also
// be restored with stateful: true — LXD otherwise performs a disk-only
// restore and silently discards the saved memory.
func newSnapshotStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	name, err := requiredString(c, "name", "snapshot name is required")
	if err != nil {
		return nil, err
	}

	action, present, err := optString(c, "action")
	if err != nil {
		return nil, err
	}
	if !present || action == "" {
		action = "create"
	}
	switch action {
	case "create", "restore", "delete":
	default:
		return nil, optionError(c, "action must be create, restore, or delete in step %q (got %q)", c.Name, action)
	}

	stateful, err := optBool(c, "stateful")
	if err != nil {
		return nil, err
	}
	if stateful && action == "delete" {
		return nil, optionError(c, "stateful applies to create and restore, not delete, in step %q", c.Name)
	}

	if _, ok := node.(ifaces.Snapshotter); !ok {
		return nil, optionError(c, "node %q does not support snapshots (supported: %s) in step %q",
			c.Node[0], nodetypes.SupportingTypes(nodetypes.CapabilitySnapshot), c.Name)
	}

	return &SnapshotStep{
		BaseStep: baseFor(c),
		node:     node,
		action:   action,
		name:     name,
		stateful: stateful,
	}, nil
}

// Run performs the configured snapshot action.
func (s *SnapshotStep) Run(updater formatters.TaskCompleter) error {
	snapshotter, ok := s.node.(ifaces.Snapshotter)
	if !ok {
		updater.Error()
		return fmt.Errorf("node does not support snapshots")
	}

	updater.Update(s.action)
	var err error
	switch s.action {
	case "create":
		err = snapshotter.Snapshot(s.name, s.stateful)
	case "restore":
		err = snapshotter.RestoreSnapshot(s.name, s.stateful)
	case "delete":
		err = snapshotter.DeleteSnapshot(s.name)
	}
	if err != nil {
		updater.Error()
		return fmt.Errorf("snapshot %s %q: %w", s.action, s.name, err)
	}

	updater.Complete()
	return nil
}
