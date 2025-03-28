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
	// ErrUnknownTestType is returned when an unknown test type is encountered
	ErrUnknownTestType = helpers.WrapError("unknown test type")
	// ErrUnknownStepType is returned when an unknown step type is encountered
	ErrUnknownStepType = helpers.WrapError("unknown step type")
	// ErrPackagesNotArray is returned when the packages field is not an array
	ErrPackagesNotArray = helpers.WrapError("packages field is not an array")
	// ErrPackageNotString is returned when a package is not a string
	ErrPackageNotString = helpers.WrapError("package is not a string")
)
