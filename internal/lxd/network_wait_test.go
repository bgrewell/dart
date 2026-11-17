package lxd

import (
	"context"
	"errors"
	"testing"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/stretchr/testify/assert"
)

// fakeNetworkServer stands in for an LXD server that handles network create
// and delete asynchronously. Each GetNetwork call consumes one scripted
// answer; the last answer repeats once the script runs out. Any other
// InstanceServer method panics on the nil embedded interface, which is what
// these tests want: nothing else should be called.
type fakeNetworkServer struct {
	lxd.InstanceServer

	createErr error
	deleteErr error
	answers   []networkAnswer

	created  []string
	deleted  []string
	getCalls int
}

type networkAnswer struct {
	status string
	err    error
}

func (f *fakeNetworkServer) CreateNetwork(network api.NetworksPost) error {
	f.created = append(f.created, network.Name)
	return f.createErr
}

func (f *fakeNetworkServer) DeleteNetwork(name string) error {
	f.deleted = append(f.deleted, name)
	return f.deleteErr
}

func (f *fakeNetworkServer) GetNetwork(name string) (*api.Network, string, error) {
	i := f.getCalls
	if i >= len(f.answers) {
		i = len(f.answers) - 1
	}
	f.getCalls++
	answer := f.answers[i]
	if answer.err != nil {
		return nil, "", answer.err
	}
	return &api.Network{Name: name, Status: answer.status}, "", nil
}

// shortenNetworkWait makes the polling fast enough for a unit test and
// restores the real values afterwards.
func shortenNetworkWait(t *testing.T) {
	t.Helper()
	interval, timeout := networkPollInterval, networkWaitTimeout
	networkPollInterval, networkWaitTimeout = time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { networkPollInterval, networkWaitTimeout = interval, timeout })
}

var (
	errNetworkNotFound = api.StatusErrorf(404, "Network not found")
	errServerDown      = errors.New("connection refused")
)

func TestCreateNetworkWaitsUntilCreated(t *testing.T) {
	tests := []struct {
		name      string
		createErr error
		answers   []networkAnswer
		wantErr   string
		wantGets  int
	}{
		{
			name:     "already created when the request returns",
			answers:  []networkAnswer{{status: api.NetworkStatusCreated}},
			wantGets: 1,
		},
		{
			name: "not found at first, then created",
			answers: []networkAnswer{
				{err: errNetworkNotFound},
				{err: errNetworkNotFound},
				{status: api.NetworkStatusCreated},
			},
			wantGets: 3,
		},
		{
			name: "pending, then created",
			answers: []networkAnswer{
				{status: api.NetworkStatusPending},
				{status: api.NetworkStatusCreated},
			},
			wantGets: 2,
		},
		{
			name:      "create request rejected",
			createErr: errors.New("already exists"),
			answers:   []networkAnswer{{status: api.NetworkStatusCreated}},
			wantErr:   "failed to create network dart-net: already exists",
			wantGets:  0,
		},
		{
			name:     "network ends up errored",
			answers:  []networkAnswer{{err: errNetworkNotFound}, {status: api.NetworkStatusErrored}},
			wantErr:  "network dart-net was created in the Errored state",
			wantGets: 2,
		},
		{
			name:     "server stops answering",
			answers:  []networkAnswer{{err: errNetworkNotFound}, {err: errServerDown}},
			wantErr:  "connection refused",
			wantGets: 2,
		},
		{
			name:    "never appears",
			answers: []networkAnswer{{err: errNetworkNotFound}},
			wantErr: "timed out after 50ms waiting for network dart-net to be created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortenNetworkWait(t)
			server := &fakeNetworkServer{createErr: tt.createErr, answers: tt.answers}

			err := CreateNetwork(context.Background(), server, "dart-net", "bridge", nil)

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
			assert.Equal(t, []string{"dart-net"}, server.created)
			if tt.wantGets > 0 || tt.createErr != nil {
				assert.Equal(t, tt.wantGets, server.getCalls)
			} else {
				assert.Greater(t, server.getCalls, 1, "should keep polling until the timeout")
			}
		})
	}
}

func TestDeleteNetworkWaitsUntilRemoved(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		answers   []networkAnswer
		wantErr   string
		wantGets  int
	}{
		{
			name:     "already gone when the request returns",
			answers:  []networkAnswer{{err: errNetworkNotFound}},
			wantGets: 1,
		},
		{
			name: "still present at first, then gone",
			answers: []networkAnswer{
				{status: api.NetworkStatusCreated},
				{status: api.NetworkStatusCreated},
				{err: errNetworkNotFound},
			},
			wantGets: 3,
		},
		{
			name:      "delete request rejected",
			deleteErr: errors.New("network is in use"),
			answers:   []networkAnswer{{status: api.NetworkStatusCreated}},
			wantErr:   "failed to delete network dart-net: network is in use",
			wantGets:  0,
		},
		{
			name:     "server stops answering",
			answers:  []networkAnswer{{status: api.NetworkStatusCreated}, {err: errServerDown}},
			wantErr:  "connection refused",
			wantGets: 2,
		},
		{
			name:    "never goes away",
			answers: []networkAnswer{{status: api.NetworkStatusCreated}},
			wantErr: "timed out after 50ms waiting for network dart-net to be removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortenNetworkWait(t)
			server := &fakeNetworkServer{deleteErr: tt.deleteErr, answers: tt.answers}

			err := DeleteNetwork(context.Background(), server, "dart-net")

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
			assert.Equal(t, []string{"dart-net"}, server.deleted)
			if tt.wantGets > 0 || tt.deleteErr != nil {
				assert.Equal(t, tt.wantGets, server.getCalls)
			} else {
				assert.Greater(t, server.getCalls, 1, "should keep polling until the timeout")
			}
		})
	}
}

// A missing network must still read as "not found" to callers after a failed
// delete request, because teardown treats that as already removed.
func TestDeleteNetworkKeepsNotFoundFromTheRequest(t *testing.T) {
	shortenNetworkWait(t)
	server := &fakeNetworkServer{deleteErr: errNetworkNotFound, answers: []networkAnswer{{err: errNetworkNotFound}}}

	err := DeleteNetwork(context.Background(), server, "dart-net")

	assert.True(t, IsNotFound(err))
	assert.Zero(t, server.getCalls)
}

func TestWaitForNetworkStopsWhenTheContextIsCancelled(t *testing.T) {
	shortenNetworkWait(t)
	networkPollInterval = time.Hour
	server := &fakeNetworkServer{answers: []networkAnswer{{err: errNetworkNotFound}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForNetwork(ctx, server, "dart-net", true)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, server.getCalls)
}
