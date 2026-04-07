package platform

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Runtime represents the detected container runtime
type Runtime string

const (
	RuntimeLXD   Runtime = "lxd"
	RuntimeIncus Runtime = "incus"
)

// DetectionResult contains the detection outcome
type DetectionResult struct {
	Runtime    Runtime
	SocketPath string
}

// socketPaths defines the socket paths to check in priority order
var socketPaths = []struct {
	path    string
	runtime Runtime
}{
	{"/var/lib/incus/unix.socket", RuntimeIncus},
	{"/var/snap/lxd/common/lxd/unix.socket", RuntimeLXD},
	{"/var/lib/lxd/unix.socket", RuntimeLXD},
}

var (
	cachedResult *DetectionResult
	cacheMutex   sync.RWMutex
)

// DetectRuntime auto-detects whether LXD or Incus is available on the system.
// It checks socket paths in priority order: Incus first, then LXD snap, then LXD native.
// The result is cached for the duration of the process.
func DetectRuntime() (*DetectionResult, error) {
	// Check cache first
	cacheMutex.RLock()
	if cachedResult != nil {
		result := cachedResult
		cacheMutex.RUnlock()
		return result, nil
	}
	cacheMutex.RUnlock()

	// Detect runtime
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if cachedResult != nil {
		return cachedResult, nil
	}

	for _, s := range socketPaths {
		if isSocketAccessible(s.path) {
			cachedResult = &DetectionResult{
				Runtime:    s.runtime,
				SocketPath: s.path,
			}
			return cachedResult, nil
		}
	}

	return nil, fmt.Errorf("no LXD or Incus installation detected; checked paths: /var/lib/incus/unix.socket, /var/snap/lxd/common/lxd/unix.socket, /var/lib/lxd/unix.socket")
}

// isSocketAccessible checks if a Unix socket exists and is accessible
func isSocketAccessible(path string) bool {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if it's a socket
	if info.Mode()&os.ModeSocket == 0 {
		return false
	}

	// Try to connect to verify it's actually usable
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// ClearCache clears the cached detection result. Useful for testing.
func ClearCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cachedResult = nil
}
