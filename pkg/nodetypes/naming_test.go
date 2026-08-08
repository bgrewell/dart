package nodetypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The platform object's name defaults to the node name, so a suite's
// containers and instances are findable by the name the YAML uses.
func TestPlatformNamesDefaultToNodeName(t *testing.T) {
	docker := &DockerNode{name: "web"}
	assert.Equal(t, "web", docker.containerName())

	lxd := &LxdNode{name: "db"}
	assert.Equal(t, "db", lxd.instanceName())
}

// An explicit name decouples the platform identifier from node identity,
// for suites that must match an externally fixed name.
func TestPlatformNamesCanBeOverridden(t *testing.T) {
	docker := &DockerNode{
		name:    "web",
		options: DockerNodeOpts{ContainerName: "acme-web-01"},
	}
	assert.Equal(t, "acme-web-01", docker.containerName())

	lxd := &LxdNode{
		name:    "db",
		options: LxdNodeOpts{InstanceName: "acme-db-01"},
	}
	assert.Equal(t, "acme-db-01", lxd.instanceName())
}

// command and entrypoint are what let an image whose default command exits
// immediately — a bare distribution image — host a node at all.
func TestDockerCommandAndEntrypointAccepted(t *testing.T) {
	cfg := &config.NodeConfig{
		Name: "shellbox",
		Type: "docker",
		Options: map[string]interface{}{
			"image":      "ubuntu:24.04",
			"command":    []interface{}{"sleep", "infinity"},
			"entrypoint": []interface{}{"/bin/sh", "-c"},
		},
	}
	require.NoError(t, ValidateNodeOptions(cfg))

	var opts DockerNodeOpts
	require.NoError(t, decodeNodeOptions(cfg.Options, &opts))
	assert.Equal(t, []string{"sleep", "infinity"}, opts.Command)
	assert.Equal(t, []string{"/bin/sh", "-c"}, opts.Entrypoint)
}

// A misspelled option previously decoded to nothing and read as configured.
func TestUnknownNodeOptionIsRejected(t *testing.T) {
	err := ValidateNodeOptions(&config.NodeConfig{
		Name: "web", Type: "docker",
		Options: map[string]interface{}{"image": "nginx", "entrypoints": []interface{}{"/bin/sh"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown option "entrypoints"`)
	// The message names what the type does accept, including the spelling
	// the author meant
	assert.Contains(t, err.Error(), "entrypoint")

	// A key that is valid on another node type is still wrong here
	err = ValidateNodeOptions(&config.NodeConfig{
		Name: "web", Type: "ssh",
		Options: map[string]interface{}{"host": "h", "pass": "p", "image": "nginx", "insecure_skip_host_key": true},
	})
	assert.ErrorContains(t, err, `unknown option "image"`)
}

// The cross-node constraints must be reachable without constructing nodes,
// which is what lets --check report them.
func TestValidateNodeSet(t *testing.T) {
	err := ValidateNodeSet([]*config.NodeConfig{
		{Name: "a", Type: "local"},
		{Name: "b", Type: "local"},
	})
	assert.ErrorContains(t, err, "only one local node allowed")

	err = ValidateNodeSet([]*config.NodeConfig{
		{Name: "a", Type: "docker"},
		{Name: "a", Type: "docker"},
	})
	assert.ErrorContains(t, err, `duplicate node name "a"`)

	assert.NoError(t, ValidateNodeSet([]*config.NodeConfig{
		{Name: "a", Type: "local"},
		{Name: "b", Type: "docker"},
	}))
}

// A node attaches to a network; it does not define one. subnet here
// consumed nothing, so a suite that set it read as though the node's
// addressing were configured.
func TestNodeLevelSubnetIsRejected(t *testing.T) {
	err := ValidateNodeOptions(&config.NodeConfig{
		Name: "box", Type: "lxd",
		Options: map[string]interface{}{
			"image": "ubuntu:24.04",
			"networks": []interface{}{
				map[string]interface{}{"name": "testnet", "subnet": "10.5.0.0/24"},
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sets subnet")
	assert.Contains(t, err.Error(), "lxd.networks")

	// name and ip remain valid on a node-level entry
	assert.NoError(t, ValidateNodeOptions(&config.NodeConfig{
		Name: "box", Type: "lxd",
		Options: map[string]interface{}{
			"image": "ubuntu:24.04",
			"networks": []interface{}{
				map[string]interface{}{"name": "testnet", "ip": "10.5.0.9"},
			},
		},
	}))
}
