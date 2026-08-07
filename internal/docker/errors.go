package docker

import (
	"strings"

	cerrdefs "github.com/containerd/errdefs"
)

// IsNotFound reports whether err indicates a missing Docker resource. The
// SDK returns typed errdefs errors; the string forms cover wrapped errors
// and older daemons ("No such container/network/image", "not found").
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if cerrdefs.IsNotFound(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such") || strings.Contains(msg, "not found")
}
