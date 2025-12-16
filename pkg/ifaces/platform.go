package ifaces

// PlatformManager is an interface representing a platform-specific resource manager
// (e.g., Docker, LXD, Incus) that handles the setup and teardown of platform resources
// such as networks, images, and profiles. This abstraction allows the controller to
// manage multiple platforms without being coupled to any specific implementation.
type PlatformManager interface {
	// Configured returns true if the platform manager has been configured
	// with resources that need to be set up and torn down.
	Configured() bool

	// Setup creates and configures platform-specific resources (networks, images, profiles, etc.)
	// This is called before individual node setup.
	Setup() error

	// Teardown removes platform-specific resources created during Setup.
	// This is called after individual node teardown.
	Teardown() error

	// Name returns a human-readable name for this platform manager (e.g., "docker", "lxd")
	Name() string
}
