package ifaces

// ContainerWrapper is an interface for container/instance management systems
// like Docker, LXD/LXC, Incus, K8s, etc. It provides a unified interface for
// setup, teardown, and configuration checking across different container runtimes.
type ContainerWrapper interface {
	// Configured returns true if the wrapper has been configured with settings
	Configured() bool

	// Setup performs initial configuration such as creating networks, images, profiles, etc.
	Setup() error

	// Teardown cleans up resources created during setup
	Teardown() error
}
