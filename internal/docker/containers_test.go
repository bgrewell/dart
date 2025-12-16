package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

// MockClient simulates Docker client operations for testing
type MockClient struct {
	client.Client
	Containers []types.Container
}

func (m *MockClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	result := []types.Container{}

	for _, c := range m.Containers {
		if options.Filters.Contains("name") && !contains(c.Names, options.Filters.Get("name")[0]) {
			continue
		}
		if options.Filters.Contains("status") && c.State != options.Filters.Get("status")[0] {
			continue
		}
		if options.Filters.Contains("ancestor") && c.Image != options.Filters.Get("ancestor")[0] {
			continue
		}
		if options.Filters.Contains("label") && !containsLabel(c.Labels, options.Filters.Get("label")[0]) {
			continue
		}
		result = append(result, c)
	}

	return result, nil
}

func contains(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}

func containsLabel(labels map[string]string, label string) bool {
	for key, value := range labels {
		if key+"="+value == label {
			return true
		}
	}
	return false
}

func TestListContainers(t *testing.T) {
	ctx := context.Background()

	mockContainers := []types.Container{
		{ID: "1", Names: []string{"container1"}, Image: "nginx", Status: "running", Labels: map[string]string{"env": "prod"}},
		{ID: "2", Names: []string{"container2"}, Image: "alpine", Status: "dead", Labels: map[string]string{}},
		{ID: "3", Names: []string{"container3"}, Image: "fake", Status: "exited", Labels: map[string]string{}},
	}
	mockClient := &MockClient{Containers: mockContainers}

	filter := &ContainerFilter{Name: "container1", Status: "running", Image: "nginx", Label: "env=prod"}
	containers, err := ListContainers(ctx, mockClient, true, filter)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(containers))
	assert.Equal(t, "1", containers[0].ID)
}

func TestListContainersNoFilter(t *testing.T) {
	ctx := context.Background()

	mockContainers := []types.Container{
		{ID: "1", Names: []string{"container1"}},
		{ID: "2", Names: []string{"container2"}},
	}
	mockClient := &MockClient{Containers: mockContainers}

	containers, err := ListContainers(ctx, mockClient, true, nil)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(containers))
}

func TestContainerReadinessConfig(t *testing.T) {
	config := &ContainerReadinessConfig{
		Timeout:      5 * time.Minute,
		PollInterval: 3 * time.Second,
	}

	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, 3*time.Second, config.PollInterval)
}

func TestDefaultContainerReadinessConfig(t *testing.T) {
	config := DefaultContainerReadinessConfig()

	assert.Equal(t, 2*time.Minute, config.Timeout)
	assert.Equal(t, 1*time.Second, config.PollInterval)
}
