package lxd

import (
	"net/http"
	"strings"

	"github.com/canonical/lxd/shared/api"
)

// IsNotFound reports whether err indicates a missing LXD resource. Both the
// typed status error and the string form are checked: the API returns typed
// errors for most paths but plain "not found" strings for some (and older
// servers).
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
