package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pullRecorder reports whether an image is present and records pull calls.
type pullRecorder struct {
	client.Client
	present map[string]bool
	pulled  []string
	pullErr error
}

func (p *pullRecorder) ImageInspect(ctx context.Context, ref string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
	if p.present[ref] {
		return image.InspectResponse{ID: "sha256:abc"}, nil
	}
	return image.InspectResponse{}, notFoundErr{}
}

func (p *pullRecorder) ImagePull(ctx context.Context, ref string, _ image.PullOptions) (io.ReadCloser, error) {
	p.pulled = append(p.pulled, ref)
	if p.pullErr != nil {
		return nil, p.pullErr
	}
	return io.NopCloser(strings.NewReader(`{"status":"Pulling"}`)), nil
}

// notFoundErr satisfies the daemon's not-found classification.
type notFoundErr struct{}

func (notFoundErr) Error() string { return "Error: No such image: nginx:alpine" }
func (notFoundErr) NotFound()     {}

func wrapperWith(rec *pullRecorder, cfg *config.DockerConfig) *Wrapper {
	return &Wrapper{cli: rec, cfg: cfg, containerNamesToId: map[string]string{}, networkNamesToId: map[string]string{}}
}

// A missing image is fetched, rather than failing at container creation with
// the daemon's "No such image".
func TestEnsureImagePullsWhenAbsent(t *testing.T) {
	rec := &pullRecorder{present: map[string]bool{}}
	require.NoError(t, wrapperWith(rec, nil).EnsureImage("nginx:alpine"))
	assert.Equal(t, []string{"nginx:alpine"}, rec.pulled)
}

// An image already present is not re-fetched, so a suite keeps the copy it
// has rather than silently moving to a newer one behind the same tag.
func TestEnsureImageDoesNotRefetch(t *testing.T) {
	rec := &pullRecorder{present: map[string]bool{"nginx:alpine": true}}
	require.NoError(t, wrapperWith(rec, nil).EnsureImage("nginx:alpine"))
	assert.Empty(t, rec.pulled)
}

// An image the suite builds has no registry to pull from.
func TestEnsureImageSkipsSuiteBuiltImages(t *testing.T) {
	cfg := &config.DockerConfig{Images: []*config.ImageConfig{
		{Name: "test_server", Tag: "latest"},
		{Name: "test_client"},
	}}
	rec := &pullRecorder{present: map[string]bool{}}
	w := wrapperWith(rec, cfg)

	require.NoError(t, w.EnsureImage("test_server:latest"))
	require.NoError(t, w.EnsureImage("test_client"))
	assert.Empty(t, rec.pulled, "suite-built images must not be pulled")

	// An unrelated image still is
	require.NoError(t, w.EnsureImage("redis:7"))
	assert.Equal(t, []string{"redis:7"}, rec.pulled)
}

// A failed pull is reported against the image, not swallowed.
func TestEnsureImageReportsPullFailure(t *testing.T) {
	rec := &pullRecorder{present: map[string]bool{}, pullErr: errors.New("unauthorized")}
	err := wrapperWith(rec, nil).EnsureImage("private/thing:1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not pull image private/thing:1")
	assert.Contains(t, err.Error(), "unauthorized")
}
