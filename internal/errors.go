package internal

import (
	"github.com/bgrewell/dart/internal/helpers"
)

var (
	// ErrNotImplemented is returned when a method is not implemented
	ErrNotImplemented = helpers.WrapError("not implemented")
	// ErrNodeAlreadyExists is returned when a node with the same name already exists
	ErrNodeAlreadyExists = helpers.WrapError("node already exists")
	// ErrLocalNodeAlreadyExists is returned when a local node already exists
	ErrLocalNodeAlreadyExists = helpers.WrapError("local node already exists")
	// ErrNodeNotFound is returned when a node is not found
	ErrNodeNotFound = helpers.WrapError("node not found")
	// ErrUnknownCheckType is returned when an unknown check type is encountered
	ErrUnknownCheckType = helpers.WrapError("unknown check type")
	// ErrUnknownNodeType is returned when an unknown node type is encountered
	ErrUnknownNodeType = helpers.WrapError("unknown node type")
)
