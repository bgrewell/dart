package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingClient captures the create call and any follow-up attachments, so
// the wiring can be asserted without a daemon.
type recordingClient struct {
	client.Client
	createdNetworking *network.NetworkingConfig
	connected         []connectCall
}

type connectCall struct {
	networkID   string
	containerID string
	settings    *network.EndpointSettings
}

func (c *recordingClient) ContainerCreate(ctx context.Context, cfg *container.Config, host *container.HostConfig,
	net *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error) {
	c.createdNetworking = net
	return container.CreateResponse{ID: "container-id"}, nil
}

func (c *recordingClient) NetworkConnect(ctx context.Context, networkID, containerID string, settings *network.EndpointSettings) error {
	c.connected = append(c.connected, connectCall{networkID, containerID, settings})
	return nil
}

func newRecordingWrapper() (*Wrapper, *recordingClient) {
	rec := &recordingClient{}
	return &Wrapper{
		cli:                rec,
		containerNamesToId: map[string]string{},
		networkNamesToId:   map[string]string{},
	}, rec
}

// A declared network must actually reach the container. It used to be parsed
// and dropped, leaving every container on the default bridge — so a suite
// asserting isolation passed because nothing was ever separated.
func TestCreateContainerAttachesDeclaredNetwork(t *testing.T) {
	w, rec := newRecordingWrapper()

	require.NoError(t, w.CreateContainer("web", "web", "nginx:alpine",
		WithNetworks([]NetworkAttachment{{Name: "frontend"}})))

	require.NotNil(t, rec.createdNetworking)
	require.Contains(t, rec.createdNetworking.EndpointsConfig, "frontend")
	assert.Empty(t, rec.connected, "a single network needs no follow-up connect")
}

// A static address is what makes a documented `ip:` mean anything.
func TestCreateContainerAppliesStaticAddress(t *testing.T) {
	w, rec := newRecordingWrapper()

	require.NoError(t, w.CreateContainer("web", "web", "nginx:alpine",
		WithNetworks([]NetworkAttachment{{Name: "frontend", IPv4: "172.30.0.10"}})))

	endpoint := rec.createdNetworking.EndpointsConfig["frontend"]
	require.NotNil(t, endpoint.IPAMConfig)
	assert.Equal(t, "172.30.0.10", endpoint.IPAMConfig.IPv4Address)
}

// Docker accepts one endpoint at creation, so additional networks have to be
// connected afterwards or they are silently lost.
func TestCreateContainerConnectsAdditionalNetworks(t *testing.T) {
	w, rec := newRecordingWrapper()

	require.NoError(t, w.CreateContainer("web", "web", "nginx:alpine",
		WithNetworks([]NetworkAttachment{
			{Name: "frontend", IPv4: "172.30.0.10"},
			{Name: "backend"},
			{Name: "mgmt", IPv4: "172.31.0.10"},
		})))

	// First at creation
	require.Len(t, rec.createdNetworking.EndpointsConfig, 1)
	require.Contains(t, rec.createdNetworking.EndpointsConfig, "frontend")

	// The rest connected, in order, against the created container
	require.Len(t, rec.connected, 2)
	assert.Equal(t, "backend", rec.connected[0].networkID)
	assert.Equal(t, "container-id", rec.connected[0].containerID)
	assert.Nil(t, rec.connected[0].settings.IPAMConfig, "no address requested")

	assert.Equal(t, "mgmt", rec.connected[1].networkID)
	require.NotNil(t, rec.connected[1].settings.IPAMConfig)
	assert.Equal(t, "172.31.0.10", rec.connected[1].settings.IPAMConfig.IPv4Address)
}

// A node that declares no networks keeps the previous behaviour.
func TestCreateContainerWithoutNetworksIsUnchanged(t *testing.T) {
	w, rec := newRecordingWrapper()

	require.NoError(t, w.CreateContainer("web", "web", "nginx:alpine"))

	assert.Empty(t, rec.createdNetworking.EndpointsConfig)
	assert.Empty(t, rec.connected)
}
