package steptypes

import (
	"os"
	"testing"
	"time"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeStep(t *testing.T, stepType string, options map[string]interface{}) (ifaces.Step, error) {
	t.Helper()
	return makeStepOn(t, nodetypes.NewMockNode(), stepType, options)
}

// Every documented step type must be constructible.
func TestAllStepTypesWired(t *testing.T) {
	cases := map[string]map[string]interface{}{
		TypeSimulated:    {"time": 1},
		TypeExecute:      {"command": "true"},
		TypeApt:          {"packages": []interface{}{"curl"}},
		TypeFileCreate:   {"path": "/tmp/x"},
		TypeFileWrite:    {"path": "/tmp/x", "contents": "y"},
		TypeFileDelete:   {"path": "/tmp/x"},
		TypeFileEdit:     {"path": "/tmp/x", "operation": "replace", "match": "a", "content": "b"},
		TypeFileExists:   {"path": "/tmp/x"},
		TypeFileRead:     {"path": "/tmp/x"},
		TypeHTTPRequest:  {"url": "http://localhost/health"},
		TypeDNSRequest:   {"hostname": "localhost"},
		TypeServiceCheck: {"service": "nginx"},
	}
	for stepType, options := range cases {
		_, err := makeStep(t, stepType, options)
		assert.NoError(t, err, stepType)
	}
}

// file_write is an alias for file_create.
func TestFileWriteAlias(t *testing.T) {
	step, err := makeStep(t, TypeFileWrite, map[string]interface{}{"path": "/tmp/x"})
	require.NoError(t, err)
	assert.IsType(t, &FileCreateStep{}, step)
}

func TestFactoryValidationErrors(t *testing.T) {
	cases := map[string]struct {
		stepType string
		options  map[string]interface{}
		contains string
	}{
		"simulated missing time":     {TypeSimulated, map[string]interface{}{}, "time is required"},
		"simulated non-numeric time": {TypeSimulated, map[string]interface{}{"time": "soon"}, "time must be a number"},
		"simulated negative time":    {TypeSimulated, map[string]interface{}{"time": -1}, "non-negative"},
		"apt missing packages":       {TypeApt, map[string]interface{}{}, "packages field is required"},
		"apt non-list packages":      {TypeApt, map[string]interface{}{"packages": "curl"}, "must be an array"},
		"create bad contents type":   {TypeFileCreate, map[string]interface{}{"path": "/tmp/x", "contents": 5}, "must be a string"},
		"create bad overwrite type":  {TypeFileCreate, map[string]interface{}{"path": "/tmp/x", "overwrite": "yes"}, "must be a boolean"},
		"create bad mode":            {TypeFileCreate, map[string]interface{}{"path": "/tmp/x", "mode": 999}, "file mode"},
		"edit line without number":   {TypeFileEdit, map[string]interface{}{"path": "/tmp/x", "operation": "insert", "match_type": "line", "content": "y"}, "line_number is required"},
		"http missing url":           {TypeHTTPRequest, map[string]interface{}{}, "url is required"},
		"http bad status type":       {TypeHTTPRequest, map[string]interface{}{"url": "http://x", "expected_status": "ok"}, "must be an integer"},
		"dns missing hostname":       {TypeDNSRequest, map[string]interface{}{}, "hostname is required"},
		"service missing service":    {TypeServiceCheck, map[string]interface{}{}, "service is required"},
	}
	for name, tc := range cases {
		_, err := makeStep(t, tc.stepType, tc.options)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.contains, name)
	}
}

// yaml.v3 parses `mode: 0644` as octal already, so those ints are used
// directly; bare decimals like `mode: 644` exceed 0o777 and are rejected.
func TestFileCreateModeParsing(t *testing.T) {
	valid := map[string]struct {
		raw  interface{}
		want os.FileMode
	}{
		"int from yaml 0644":   {0o644, 0o644},
		"int from yaml 0600":   {0o600, 0o600},
		"string plain":         {"644", 0o644},
		"string zero-prefixed": {"0644", 0o644},
		"string 0o-prefixed":   {"0o755", 0o755},
		"string special bits":  {"04755", 0o4755},
	}
	for name, tc := range valid {
		step, err := makeStep(t, TypeFileCreate, map[string]interface{}{"path": "/tmp/x", "mode": tc.raw})
		require.NoError(t, err, name)
		assert.Equal(t, tc.want, step.(*FileCreateStep).mode, name)
	}

	invalid := map[string]interface{}{
		"bare decimal 644":  644,
		"bare decimal 755":  755,
		"special bits int":  0o4755,
		"garbage int":       999,
		"non-octal string":  "0698",
		"out-of-range mode": "77777",
	}
	for name, raw := range invalid {
		_, err := makeStep(t, TypeFileCreate, map[string]interface{}{"path": "/tmp/x", "mode": raw})
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "file mode", name)
	}
}

func TestSimulatedFractionalSeconds(t *testing.T) {
	step, err := makeStep(t, TypeSimulated, map[string]interface{}{"time": 0.25})
	require.NoError(t, err)
	assert.Equal(t, 250*time.Millisecond, step.(*SimulatedStep).sleepTime)
}
