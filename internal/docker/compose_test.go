package docker

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestNewComposeStack tests creating a new compose stack
func TestNewComposeStack(t *testing.T) {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	stack := NewComposeStack(cli, "test-stack", "/path/to/compose.yml", "test-project")

	assert.NotNil(t, stack)
	assert.Equal(t, "test-stack", stack.Name)
	assert.Equal(t, "/path/to/compose.yml", stack.ComposeFile)
	assert.Equal(t, "test-project", stack.ProjectName)
	assert.NotNil(t, stack.containerIDs)
}

// TestNewComposeStackDefaultProjectName tests that project name defaults to stack name
func TestNewComposeStackDefaultProjectName(t *testing.T) {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	stack := NewComposeStack(cli, "test-stack", "/path/to/compose.yml", "")

	assert.NotNil(t, stack)
	assert.Equal(t, "test-stack", stack.ProjectName)
}

// TestDiscoverContainers tests the container discovery functionality
func TestDiscoverContainers(t *testing.T) {
	mockContainers := []types.Container{
		{
			ID:    "container1",
			Names: []string{"/test-project-web-1"},
			Labels: map[string]string{
				"com.docker.compose.project": "test-project",
				"com.docker.compose.service": "web",
			},
		},
		{
			ID:    "container2",
			Names: []string{"/test-project-db-1"},
			Labels: map[string]string{
				"com.docker.compose.project": "test-project",
				"com.docker.compose.service": "db",
			},
		},
	}

	mockClient := &MockClient{Containers: mockContainers}

	stack := &ComposeStack{
		Name:         "test-stack",
		ComposeFile:  "/path/to/compose.yml",
		ProjectName:  "test-project",
		cli:          mockClient,
		containerIDs: make(map[string]string),
	}

	err := stack.discoverContainers()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(stack.containerIDs))
	assert.Equal(t, "container1", stack.containerIDs["web"])
	assert.Equal(t, "container2", stack.containerIDs["db"])
}

// TestGetServiceContainerID tests retrieving container ID by service name
func TestGetServiceContainerID(t *testing.T) {
	stack := &ComposeStack{
		containerIDs: map[string]string{
			"web": "container1",
			"db":  "container2",
		},
	}

	id, err := stack.GetServiceContainerID("web")
	assert.Nil(t, err)
	assert.Equal(t, "container1", id)

	id, err = stack.GetServiceContainerID("db")
	assert.Nil(t, err)
	assert.Equal(t, "container2", id)

	_, err = stack.GetServiceContainerID("nonexistent")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "service 'nonexistent' not found")
}

// TestListServices tests listing all services in the stack
func TestListServices(t *testing.T) {
	stack := &ComposeStack{
		containerIDs: map[string]string{
			"web": "container1",
			"db":  "container2",
		},
	}

	services := stack.ListServices()
	assert.Equal(t, 2, len(services))
	assert.Contains(t, services, "web")
	assert.Contains(t, services, "db")
}
