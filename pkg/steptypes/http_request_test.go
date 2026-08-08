package steptypes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPRequestStep verifies HTTP response handling from both vantages.
// Against a local node the two must agree, which is what makes the default
// safe to change.
func TestHTTPRequestStep(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Dart") != "yes" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	for _, from := range []string{"node", "host"} {
		t.Run("from="+from, func(t *testing.T) {
			node := ifaces.Node(nodetypes.NewMockNode())
			if from == "node" {
				skipWithoutTool(t, "curl")
				node = localNode(t)
			}

			step, err := makeStepOn(t, node, TypeHTTPRequest, map[string]interface{}{
				"url":             server.URL,
				"headers":         map[string]interface{}{"X-Dart": "yes"},
				"expected_status": 200,
				"expected_body":   "success",
				"timeout":         5,
				"from":            from,
			})
			require.NoError(t, err)
			require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

			// A missing header produces 403, and the status check catches it
			denied, err := makeStepOn(t, node, TypeHTTPRequest, map[string]interface{}{
				"url": server.URL, "timeout": 5, "from": from,
			})
			require.NoError(t, err)
			err = denied.Run(formatters.NewMockTaskCompleter())
			assert.ErrorContains(t, err, "got 403")

			// A body that never appears fails rather than passing silently
			wrongBody, err := makeStepOn(t, node, TypeHTTPRequest, map[string]interface{}{
				"url":           server.URL,
				"headers":       map[string]interface{}{"X-Dart": "yes"},
				"expected_body": "not in the response",
				"timeout":       5,
				"from":          from,
			})
			require.NoError(t, err)
			assert.Error(t, wrongBody.Run(formatters.NewMockTaskCompleter()))
		})
	}
}

// An unreachable endpoint is an error from either vantage, never a pass.
func TestHTTPRequestStepUnreachable(t *testing.T) {
	skipWithoutTool(t, "curl")
	step, err := makeStepOn(t, localNode(t), TypeHTTPRequest, map[string]interface{}{
		// Reserved for documentation, so nothing answers
		"url": "http://192.0.2.1:9/", "timeout": 2,
	})
	require.NoError(t, err)
	assert.Error(t, step.Run(formatters.NewMockTaskCompleter()))
}
