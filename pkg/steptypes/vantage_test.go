package steptypes

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipWithoutTool(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s is required for this test", tool)
	}
}

// Network steps default to the node they name, not the controller.
func TestNetworkStepsDefaultToNode(t *testing.T) {
	httpStep, err := makeStep(t, TypeHTTPRequest, map[string]interface{}{"url": "http://localhost/"})
	require.NoError(t, err)
	assert.Equal(t, VantageNode, httpStep.(*HTTPRequestStep).from)

	dnsStep, err := makeStep(t, TypeDNSRequest, map[string]interface{}{"hostname": "localhost"})
	require.NoError(t, err)
	assert.Equal(t, VantageNode, dnsStep.(*DNSRequestStep).from)
}

func TestStepVantageValidation(t *testing.T) {
	_, err := makeStep(t, TypeHTTPRequest, map[string]interface{}{
		"url": "http://localhost/", "from": "somewhere",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `from must be "node" or "host"`)

	step, err := makeStep(t, TypeDNSRequest, map[string]interface{}{
		"hostname": "localhost", "from": "host",
	})
	require.NoError(t, err)
	assert.Equal(t, VantageHost, step.(*DNSRequestStep).from)
}
