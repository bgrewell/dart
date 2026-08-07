package report

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleReport() *Report {
	return &Report{
		Suite: "sample", Passed: 1, Failed: 1, Skipped: 1, Ran: 0,
		Duration: 3500 * time.Millisecond,
		Tests: []TestRecord{
			{Name: "passes", Node: "n1", Status: StatusPass, Duration: time.Second},
			{Name: "fails", Node: "n1", Status: StatusFail, Duration: 2 * time.Second,
				Failures: []string{"exit_code: Expected: 0 Actual: 2"}},
			{Name: "skipped", Node: "n2", Status: StatusSkip, Reason: "skip_if condition met"},
		},
	}
}

func TestParseSpec(t *testing.T) {
	spec, err := ParseSpec("junit:out.xml")
	require.NoError(t, err)
	assert.Equal(t, Spec{Format: "junit", Path: "out.xml"}, spec)

	spec, err = ParseSpec("json:/tmp/r.json")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/r.json", spec.Path)

	_, err = ParseSpec("junit")
	assert.ErrorContains(t, err, "format:path")
	_, err = ParseSpec("yaml:x.yaml")
	assert.ErrorContains(t, err, "unknown report format")
}

func TestJUnitRender(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.xml")
	require.NoError(t, Write(Spec{Format: "junit", Path: path}, sampleReport()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var suite struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Skipped  int      `xml:"skipped,attr"`
		Cases    []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Body string `xml:",chardata"`
			} `xml:"failure"`
			Skipped *struct {
				Message string `xml:"message,attr"`
			} `xml:"skipped"`
		} `xml:"testcase"`
	}
	require.NoError(t, xml.Unmarshal(data, &suite))
	assert.Equal(t, 3, suite.Tests)
	assert.Equal(t, 1, suite.Failures)
	assert.Equal(t, 1, suite.Skipped)
	require.Len(t, suite.Cases, 3)
	require.NotNil(t, suite.Cases[1].Failure)
	assert.Contains(t, suite.Cases[1].Failure.Body, "exit_code")
	require.NotNil(t, suite.Cases[2].Skipped)
	assert.Contains(t, suite.Cases[2].Skipped.Message, "skip_if")
}

func TestJSONRender(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.json")
	require.NoError(t, Write(Spec{Format: "json", Path: path}, sampleReport()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded Report
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "sample", decoded.Suite)
	assert.Equal(t, 1, decoded.Failed)
	assert.InDelta(t, 3.5, decoded.Seconds, 0.001)
	require.Len(t, decoded.Tests, 3)
	assert.InDelta(t, 2.0, decoded.Tests[1].Seconds, 0.001)
	assert.Equal(t, StatusSkip, decoded.Tests[2].Status)
}

// Control characters (ANSI escapes from command output) must not reach the
// XML — CI parsers reject files containing them.
func TestJUnitSanitizesControlCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ansi.xml")
	r := &Report{Suite: "s", Failed: 1, Tests: []TestRecord{
		{Name: "colored \x1b[31mfail\x1b[0m", Node: "n", Status: StatusFail,
			Failures: []string{"match: got \x1b[32mgreen\x1b[0m text\x00"}},
	}}
	require.NoError(t, Write(Spec{Format: "junit", Path: path}, r))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "\x1b")
	assert.NotContains(t, string(data), "\x00")

	var parsed struct {
		XMLName xml.Name `xml:"testsuite"`
	}
	assert.NoError(t, xml.Unmarshal(data, &parsed), "output must be parseable XML")
}

func TestFromRecordsCounts(t *testing.T) {
	r := FromRecords("s", []TestRecord{
		{Status: StatusPass}, {Status: StatusPass},
		{Status: StatusFail}, {Status: StatusError},
		{Status: StatusSkip}, {Status: StatusRan},
	}, time.Second)
	assert.Equal(t, 2, r.Passed)
	assert.Equal(t, 2, r.Failed, "errors count as failures in totals")
	assert.Equal(t, 1, r.Skipped)
	assert.Equal(t, 1, r.Ran)
}

func TestJUnitErrorElement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "err.xml")
	r := FromRecords("s", []TestRecord{
		{Name: "infra", Node: "n", Status: StatusError, Failures: []string{"node unreachable"}},
	}, time.Second)
	require.NoError(t, Write(Spec{Format: "junit", Path: path}, r))
	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), `errors="1"`)
	assert.Contains(t, string(data), "<error")
	assert.Contains(t, string(data), "node unreachable")
}
