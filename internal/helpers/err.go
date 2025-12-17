package helpers

import (
	"fmt"
	"runtime"
)

var (
	// ErrNotImplemented is returned when a method is not implemented
	ErrNotImplemented = WrapError("not implemented")
	// ErrNodeAlreadyExists is returned when a node with the same name already exists
	ErrNodeAlreadyExists = WrapError("node already exists")
	// ErrLocalNodeAlreadyExists is returned when a local node already exists
	ErrLocalNodeAlreadyExists = WrapError("local node already exists")
	// ErrNodeNotFound is returned when a node is not found
	ErrNodeNotFound = WrapError("node not found")
	// ErrUnknownCheckType is returned when an unknown check type is encountered
	ErrUnknownCheckType = WrapError("unknown check type")
	// ErrUnknownNodeType is returned when an unknown node type is encountered
	ErrUnknownNodeType = WrapError("unknown node type")
	// ErrUnknownTestType is returned when an unknown test type is encountered
	ErrUnknownTestType = WrapError("unknown test type")
	// ErrUnknownStepType is returned when an unknown step type is encountered
	ErrUnknownStepType = WrapError("unknown step type")
	// ErrPackagesNotArray is returned when the packages field is not an array
	ErrPackagesNotArray = WrapError("packages field is not an array")
	// ErrPackageNotString is returned when a package is not a string
	ErrPackageNotString = WrapError("package is not a string")
	// ErrCommandNotString is returned when a command in an array is not a string
	ErrCommandNotString = WrapError("command is not a string")
	// ErrInvalidCommandType is returned when the command field is neither a string nor an array
	ErrInvalidCommandType = WrapError("command must be a string or array of strings")
)

// WrapError adds file and line number information to an error message
func WrapError(message string) error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	return fmt.Errorf("%s (at %s:%d)", message, file, line)
}
