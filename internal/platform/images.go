package platform

import (
	"fmt"
	"strings"
)

// ubuntuRemote is the only remote that needs rewriting on Incus. Canonical's
// simplestreams endpoint behind `ubuntu:` does not serve Incus clients, so an
// `ubuntu:` reference is redirected to the equivalent image on the
// linuxcontainers server, where Ubuntu images live under `ubuntu/<release>`
// and the cloud variant carries the `/cloud` suffix.
const ubuntuRemote = "ubuntu"

// cloudVariant is the suffix identifying the cloud-init enabled build.
const cloudVariant = "/cloud"

// TranslateImage adapts an LXD-style image reference for the running runtime.
//
// For LXD the reference is used as written. For Incus only `ubuntu:` is
// rewritten — `ubuntu:24.04` becomes `images:ubuntu/24.04/cloud`. Every other
// remote is left alone: rewriting them redirected `lxc:` and any private or
// self-hosted remote to the public linuxcontainers server, and appended a
// `/cloud` variant that most image families do not publish, so the image
// could not be found.
func TranslateImage(ref string, runtime Runtime) string {
	if runtime == RuntimeLXD {
		return ref
	}

	remote, alias, found := strings.Cut(ref, ":")
	if !found || remote != ubuntuRemote {
		return ref
	}

	// A reference that already names the variant is left as it is, so an
	// explicit `ubuntu:24.04/cloud` does not become `.../cloud/cloud`
	if strings.HasSuffix(alias, cloudVariant) {
		return fmt.Sprintf("images:%s/%s", ubuntuRemote, alias)
	}

	return fmt.Sprintf("images:%s/%s%s", ubuntuRemote, alias, cloudVariant)
}
