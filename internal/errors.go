package internal

import "errors"

var (
	// ErrNotImplemented is returned when a method is not implemented
	ErrNotImplemented = errors.New("not implemented")
	// ErrNodeAlreadyExists is returned when a node with the same name already exists
	ErrNodeAlreadyExists = errors.New("node already exists")
	// ErrLocalNodeAlreadyExists is returned when a local node already exists
	ErrLocalNodeAlreadyExists = errors.New("local node already exists")
	// ErrNodeNotFound is returned when a node is not found
	ErrNodeNotFound = errors.New("node not found")
	// ErrUnknownCheckType is returned when an unknown check type is encountered
	ErrUnknownCheckType = errors.New("unknown check type")
)
