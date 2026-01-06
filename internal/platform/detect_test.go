package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslateImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		runtime  Runtime
		expected string
	}{
		{
			name:     "LXD ubuntu image unchanged",
			image:    "ubuntu:24.04",
			runtime:  RuntimeLXD,
			expected: "ubuntu:24.04",
		},
		{
			name:     "LXD images remote unchanged",
			image:    "images:debian/12",
			runtime:  RuntimeLXD,
			expected: "images:debian/12",
		},
		{
			name:     "Incus ubuntu image translated",
			image:    "ubuntu:24.04",
			runtime:  RuntimeIncus,
			expected: "images:ubuntu/24.04",
		},
		{
			name:     "Incus images remote unchanged",
			image:    "images:debian/12",
			runtime:  RuntimeIncus,
			expected: "images:debian/12",
		},
		{
			name:     "Incus lxc remote translated",
			image:    "lxc:alpine/3.18",
			runtime:  RuntimeIncus,
			expected: "images:lxc/alpine/3.18",
		},
		{
			name:     "No colon returns unchanged for LXD",
			image:    "myimage",
			runtime:  RuntimeLXD,
			expected: "myimage",
		},
		{
			name:     "No colon returns unchanged for Incus",
			image:    "myimage",
			runtime:  RuntimeIncus,
			expected: "myimage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateImage(tt.image, tt.runtime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRuntimeConstants(t *testing.T) {
	assert.Equal(t, Runtime("lxd"), RuntimeLXD)
	assert.Equal(t, Runtime("incus"), RuntimeIncus)
}
