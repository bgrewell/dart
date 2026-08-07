package lxd

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/lxd/shared/api"
	"github.com/stretchr/testify/assert"
)

// Validation failures must surface before any server call — a nil server
// proves the checks run first.
func TestCreateBridgeNetworkValidation(t *testing.T) {
	err := CreateBridgeNetwork(context.Background(), nil, "net", "not-a-subnet", "10.0.0.1", true)
	assert.ErrorContains(t, err, "not valid CIDR")

	err = CreateBridgeNetwork(context.Background(), nil, "net", "10.0.0.0/24", "not-an-ip", true)
	assert.ErrorContains(t, err, "not a valid IP")
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, IsNotFound(nil))
	assert.False(t, IsNotFound(errors.New("connection refused")))
	assert.True(t, IsNotFound(errors.New("network dart-net not found")))
	assert.True(t, IsNotFound(api.StatusErrorf(404, "missing")))
}
