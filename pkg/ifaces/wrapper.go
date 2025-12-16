package ifaces

// EnvironmentWrapper is an interface for environment management systems
// like Docker, LXD/LXC, Incus, K8s, etc. It provides a unified interface for
// setup, teardown, and configuration checking across different runtime platforms
// that can manage containers, VMs, or other execution environments.
type EnvironmentWrapper interface {
	// Configured returns true if the wrapper has been configured with settings
	Configured() bool

	// Setup performs initial configuration such as creating networks, images, profiles, etc.
	Setup() error

	// Teardown cleans up resources created during setup
	Teardown() error
}
