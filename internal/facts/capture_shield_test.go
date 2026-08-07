package facts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var shieldStore = FactStore{
	"node1": {"ip": "10.0.0.5"},
}

// Capture references must survive fact templating untouched — they resolve
// at run time via the capture store, and text/template would otherwise
// reject or mangle them.
func TestRenderTemplatePreservesCaptureRefs(t *testing.T) {
	out, err := RenderTemplate(`echo {{capture.pre_id}}`, shieldStore, "node1")
	require.NoError(t, err)
	assert.Equal(t, `echo {{capture.pre_id}}`, out)
}

func TestRenderTemplateMixedFactAndCapture(t *testing.T) {
	out, err := RenderTemplate(
		`ping {{ fact "node1" "ip" }} && echo {{capture.pre_id}}`, shieldStore, "node1")
	require.NoError(t, err)
	assert.Equal(t, `ping 10.0.0.5 && echo {{capture.pre_id}}`, out)
}

func TestRenderTemplateMultipleCaptureRefs(t *testing.T) {
	out, err := RenderTemplate(
		`compare {{capture.before}} {{ capture.after }}`, shieldStore, "node1")
	require.NoError(t, err)
	assert.Equal(t, `compare {{capture.before}} {{ capture.after }}`, out)
}

func TestGatherFactsFromMockNode(t *testing.T) {
	// Exercised through the public API in the controller tests; here just
	// verify template rendering against a store built by hand still renders
	// self-references
	out, err := RenderTemplate(`{{ fact "self" "ip" }}`, shieldStore, "node1")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.5", out)
}
