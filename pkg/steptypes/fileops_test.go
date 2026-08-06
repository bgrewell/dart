package steptypes

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allFileOps exercises the same behaviors against both implementations so
// local and shell-based file operations stay in sync. The exec variant runs
// against a shell-backed local node, mirroring how remote nodes (docker
// `sh -c`, LXD `bash -c`, SSH) interpret commands.
func allFileOps(t *testing.T) map[string]fileOps {
	t.Helper()
	ops := map[string]fileOps{
		"local": localFileOps{},
	}
	if runtime.GOOS != "windows" {
		nodeOpts := map[string]interface{}{"shell": "/bin/sh"}
		ops["exec"] = execFileOps{node: nodetypes.NewLocalNode("fileops-test", ifaces.NodeOptions(&nodeOpts))}
	}
	return ops
}

func TestFileOpsWriteReadRoundtrip(t *testing.T) {
	for name, ops := range allFileOps(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "roundtrip.txt")
			contents := "line one\nline 'two' with $pecial \"chars\"\n"

			require.NoError(t, ops.WriteFile(p, contents, 0, false, false))

			got, err := ops.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, contents, got)
		})
	}
}

func TestFileOpsWriteExclusive(t *testing.T) {
	for name, ops := range allFileOps(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "exclusive.txt")
			require.NoError(t, ops.WriteFile(p, "first", 0, false, false))

			err := ops.WriteFile(p, "second", 0, false, false)
			assert.Error(t, err, "exclusive write over an existing file must fail")

			require.NoError(t, ops.WriteFile(p, "second", 0, true, false))
			got, err := ops.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, "second", got)
		})
	}
}

func TestFileOpsWriteCreateDirAndMode(t *testing.T) {
	for name, ops := range allFileOps(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "sub", "dir", "mode.txt")
			require.NoError(t, ops.WriteFile(p, "x", 0600, false, true))

			mode, err := ops.FileMode(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), mode)
		})
	}
}

func TestFileOpsExistsAndDelete(t *testing.T) {
	for name, ops := range allFileOps(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "victim.txt")

			exists, err := ops.Exists(p)
			require.NoError(t, err)
			assert.False(t, exists)

			require.NoError(t, ops.WriteFile(p, "x", 0, false, false))
			exists, err = ops.Exists(p)
			require.NoError(t, err)
			assert.True(t, exists)

			require.NoError(t, ops.DeleteFile(p))
			exists, err = ops.Exists(p)
			require.NoError(t, err)
			assert.False(t, exists)

			assert.Error(t, ops.DeleteFile(p), "deleting a missing file must fail")
		})
	}
}

func TestFileOpsQuotedPaths(t *testing.T) {
	for name, ops := range allFileOps(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "it's a file with spaces.txt")
			require.NoError(t, ops.WriteFile(p, "quoted", 0, false, false))

			got, err := ops.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, "quoted", got)

			require.NoError(t, ops.DeleteFile(p))
		})
	}
}

func TestFileOpsForSelection(t *testing.T) {
	assert.IsType(t, localFileOps{}, fileOpsFor(nil))
	assert.IsType(t, localFileOps{}, fileOpsFor(nodetypes.NewLocalNode("local", nil)))
	assert.IsType(t, execFileOps{}, fileOpsFor(nodetypes.NewMockNode()))
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'plain'", shellQuote("plain"))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
	assert.Equal(t, "'a b;rm -rf $HOME`x`'", shellQuote("a b;rm -rf $HOME`x`"))
}
