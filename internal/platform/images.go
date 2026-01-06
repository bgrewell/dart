package platform

import (
	"fmt"
	"strings"
)

// TranslateImage converts LXD-style image references to runtime-appropriate format.
// For LXD, images are returned unchanged.
// For Incus, images are translated: "ubuntu:24.04" becomes "images:ubuntu/24.04"
func TranslateImage(ref string, runtime Runtime) string {
	if runtime == RuntimeLXD {
		return ref
	}

	// Incus translation: ubuntu:24.04 → images:ubuntu/24.04
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		// Can't parse, return as-is
		return ref
	}

	remote, alias := parts[0], parts[1]

	// If already using the images remote, no translation needed
	if remote == "images" {
		return ref
	}

	return fmt.Sprintf("images:%s/%s", remote, alias)
}
