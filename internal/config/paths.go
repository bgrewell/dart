package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveLocalPath turns a path written in a suite into an absolute path on
// the machine running DART.
//
// One convention covers every local path a suite can write: absolute paths
// are used as-is, `~` expands to the invoking user's home directory, and
// everything else is relative to the directory holding the suite file. That
// makes a suite portable — it behaves the same whether it is run from the
// repository root, from its own directory, or from a CI checkout elsewhere.
//
// suiteDir is empty for configurations built in memory rather than loaded
// from a file, in which case relative paths fall back to the process working
// directory.
func ResolveLocalPath(suiteDir, path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve %q: %w", path, err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}

	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	if suiteDir != "" {
		return filepath.Join(suiteDir, path), nil
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q: %w", path, err)
	}
	return absolute, nil
}
