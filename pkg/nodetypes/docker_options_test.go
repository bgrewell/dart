package nodetypes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A relative host path must become absolute: the Engine API treats a
// non-absolute source as a NAMED VOLUME, silently mounting an empty
// volume instead of the directory.
func TestResolveVolumesMakesHostPathsAbsolute(t *testing.T) {
	resolved, err := resolveVolumes([]string{"./fixtures:/fixtures:ro"}, "")
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	assert.True(t, filepath.IsAbs(strings.SplitN(resolved[0], ":", 2)[0]),
		"relative host paths must resolve to absolute bind mounts")
	assert.True(t, strings.HasSuffix(resolved[0], ":/fixtures:ro"), "container side preserved")
}

func TestResolveVolumesKeepsNamedVolumes(t *testing.T) {
	resolved, err := resolveVolumes([]string{"cache-data:/var/cache"}, "")
	require.NoError(t, err)
	assert.Equal(t, "cache-data:/var/cache", resolved[0], "a bare name stays a named volume")
}

func TestResolveVolumesExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	resolved, err := resolveVolumes([]string{"~/data:/data"}, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resolved[0], home))
}

func TestResolveVolumesRejectsMalformed(t *testing.T) {
	_, err := resolveVolumes([]string{"/no-container-side"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host:container")
}

// --check must catch missing required fields, so a suite does not get all
// the way to a dial or a container create before reporting the real mistake.
func TestValidateNodeOptionsRequiresRequiredFields(t *testing.T) {
	err := ValidateNodeOptions(&config.NodeConfig{
		Name: "web", Type: "docker", Options: map[string]interface{}{},
	})
	assert.ErrorContains(t, err, "image is required")

	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "remote", Type: "ssh",
		Options: map[string]interface{}{"user": "root", "insecure_skip_host_key": true, "pass": "x"},
	})
	assert.ErrorContains(t, err, "host is required")

	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "stack", Type: "docker-compose", Options: map[string]interface{}{},
	})
	assert.ErrorContains(t, err, "compose_file is required")
}

// --check must catch option problems that need no daemon or network.
func TestValidateNodeOptionsCatchesLocalProblems(t *testing.T) {
	err := ValidateNodeOptions(&config.NodeConfig{
		Name: "web", Type: "docker",
		Options: map[string]interface{}{"image": "nginx", "ports": []interface{}{"not-a-port-spec:::"}},
	})
	assert.Error(t, err)

	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "web", Type: "docker",
		Options: map[string]interface{}{"image": "nginx", "volumes": []interface{}{"/bad-spec"}},
	})
	assert.ErrorContains(t, err, "host:container")

	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "remote", Type: "ssh",
		Options: map[string]interface{}{"host": "x", "insecure_skip_host_key": true},
	})
	assert.ErrorContains(t, err, "no ssh credentials")

	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "remote", Type: "ssh",
		Options: map[string]interface{}{
			"host": "x", "pass": "p", "insecure_skip_host_key": true,
			"bastion": map[string]interface{}{"user": "u", "pass": "p"},
		},
	})
	assert.ErrorContains(t, err, "bastion host is required")

	// A well-formed config passes
	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "ok", Type: "docker",
		Options: map[string]interface{}{
			"image": "ubuntu", "ports": []interface{}{"8080:80"},
			"volumes": []interface{}{"/tmp:/tmp:ro"},
		},
	})
	assert.NoError(t, err)
}
