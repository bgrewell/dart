package docker

import (
	"errors"
	"fmt"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFound(t *testing.T) {
	assert.False(t, IsNotFound(nil))
	assert.False(t, IsNotFound(errors.New("connection refused")))
	assert.True(t, IsNotFound(errors.New("Error: No such container: dart-node")))
	assert.True(t, IsNotFound(errors.New("network dart-net not found")))
	assert.True(t, IsNotFound(cerrdefs.ErrNotFound))
	assert.True(t, IsNotFound(fmt.Errorf("could not remove container: %w", cerrdefs.ErrNotFound)))
}
