package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// LXD uses references exactly as written.
func TestTranslateImageLeavesLXDAlone(t *testing.T) {
	for _, ref := range []string{
		"ubuntu:24.04",
		"images:alpine/3.19",
		"lxc:debian/12",
		"local:my-golden-image",
		"my-image",
	} {
		assert.Equal(t, ref, TranslateImage(ref, RuntimeLXD), ref)
	}
}

// On Incus only ubuntu: is rewritten. Rewriting the others redirected them to
// the public linuxcontainers server and appended a /cloud variant most image
// families do not publish, so the image could not be found.
func TestTranslateImageOnlyRewritesUbuntu(t *testing.T) {
	cases := map[string]string{
		// The one remote that needs the shim
		"ubuntu:24.04": "images:ubuntu/24.04/cloud",
		"ubuntu:noble": "images:ubuntu/noble/cloud",
		"ubuntu:22.04": "images:ubuntu/22.04/cloud",

		// Already on the images server: untouched
		"images:ubuntu/24.04/cloud": "images:ubuntu/24.04/cloud",
		"images:alpine/3.19":        "images:alpine/3.19",

		// Other remotes keep their own server — rewriting these was the bug
		"lxc:debian/12":         "lxc:debian/12",
		"alpine:3.19":           "alpine:3.19",
		"local:my-image":        "local:my-image",
		"myremote:custom/thing": "myremote:custom/thing",

		// No remote at all: a local alias
		"my-image": "my-image",
	}
	for ref, want := range cases {
		assert.Equal(t, want, TranslateImage(ref, RuntimeIncus), ref)
	}
}

// An explicit variant must not be doubled.
func TestTranslateImageDoesNotDoubleTheVariant(t *testing.T) {
	assert.Equal(t, "images:ubuntu/24.04/cloud", TranslateImage("ubuntu:24.04/cloud", RuntimeIncus))
	assert.Equal(t, "images:ubuntu/noble/cloud", TranslateImage("ubuntu:noble/cloud", RuntimeIncus))
}
