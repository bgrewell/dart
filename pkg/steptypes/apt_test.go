package steptypes

import (
	"fmt"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
)

const aptStampCmd = "stat -c %Y /var/lib/apt/periodic/update-success-stamp"

func TestAptUpdateNeededFreshStamp(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	epoch := time.Now().Add(-1 * time.Hour).Unix()
	mockNode.SetResponse(aptStampCmd, 0, fmt.Sprintf("%d\n", epoch), "")

	step := &AptStep{node: mockNode}
	assert.False(t, step.AptUpdateNeeded())
}

func TestAptUpdateNeededStaleStamp(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	epoch := time.Now().Add(-48 * time.Hour).Unix()
	mockNode.SetResponse(aptStampCmd, 0, fmt.Sprintf("%d\n", epoch), "")

	step := &AptStep{node: mockNode}
	assert.True(t, step.AptUpdateNeeded())
}

func TestAptUpdateNeededMissingStamp(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse(aptStampCmd, 1, "", "stat: cannot statx: No such file or directory")

	step := &AptStep{node: mockNode}
	assert.True(t, step.AptUpdateNeeded())
}

func TestAptUpdateNeededGarbageOutput(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse(aptStampCmd, 0, "not a number\n", "")

	step := &AptStep{node: mockNode}
	assert.True(t, step.AptUpdateNeeded())
}

// A failing command's stderr must appear in the step error, not a reader
// struct.
func TestExecuteStepStderrInError(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("false-cmd", 2, "", "something exploded\n")

	step := &ExecuteStep{node: mockNode, commands: []string{"false-cmd"}}
	err := step.Run(formatters.NewMockTaskCompleter())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 2")
	assert.Contains(t, err.Error(), "something exploded")
}
