// Package report renders suite results into machine-readable formats for
// CI systems: JUnit XML for test panels and JSON for custom tooling.
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"
)

// Status is the outcome of one executed (or skipped) test.
type Status string

const (
	StatusPass  Status = "pass"
	StatusFail  Status = "fail"
	StatusSkip  Status = "skip"
	StatusRan   Status = "ran"   // executed with no evaluations configured
	StatusError Status = "error" // infrastructure error (node unreachable, setup failure, ...)
)

// TestRecord captures one test's outcome for reporting.
type TestRecord struct {
	Name     string        `json:"name"`
	Node     string        `json:"node"`
	Status   Status        `json:"status"`
	Duration time.Duration `json:"-"`
	Seconds  float64       `json:"duration_seconds"`
	// Failures holds "check: detail" lines for failed checks; Reason holds
	// a skip reason.
	Failures []string `json:"failures,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// FromRecords builds a Report with totals derived from the records — used
// when a run aborts early and the normal summary counting never happened.
func FromRecords(suite string, records []TestRecord, elapsed time.Duration) *Report {
	r := &Report{Suite: suite, Tests: records, Duration: elapsed}
	for _, test := range records {
		switch test.Status {
		case StatusPass:
			r.Passed++
		case StatusFail, StatusError:
			r.Failed++
		case StatusSkip:
			r.Skipped++
		case StatusRan:
			r.Ran++
		}
	}
	return r
}

// Report is the complete suite outcome.
type Report struct {
	Suite    string        `json:"suite"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Skipped  int           `json:"skipped"`
	Ran      int           `json:"ran"`
	Seconds  float64       `json:"duration_seconds"`
	Tests    []TestRecord  `json:"tests"`
	Duration time.Duration `json:"-"`
}

// Spec is a parsed --report flag value.
type Spec struct {
	Format string // "junit" or "json"
	Path   string
}

// ParseSpec parses a --report value of the form "format:path".
func ParseSpec(value string) (Spec, error) {
	format, path, found := strings.Cut(value, ":")
	if !found || path == "" {
		return Spec{}, fmt.Errorf("report spec %q must be format:path (e.g. junit:results.xml)", value)
	}
	switch format {
	case "junit", "json":
		return Spec{Format: format, Path: path}, nil
	default:
		return Spec{}, fmt.Errorf("unknown report format %q (supported: junit, json)", format)
	}
}

// Write renders the report to the spec's path.
func Write(spec Spec, r *Report) error {
	var data []byte
	var err error
	switch spec.Format {
	case "junit":
		data, err = renderJUnit(r)
	case "json":
		data, err = renderJSON(r)
	default:
		return fmt.Errorf("unknown report format %q", spec.Format)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(spec.Path, data, 0644)
}

func renderJSON(r *Report) ([]byte, error) {
	r.Seconds = r.Duration.Seconds()
	for i := range r.Tests {
		r.Tests[i].Seconds = r.Tests[i].Duration.Seconds()
	}
	return json.MarshalIndent(r, "", "  ")
}

// JUnit schema subset understood by GitHub/GitLab/Jenkins.
type junitTestsuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	Testcases []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// sanitizeXML removes runes that are invalid in XML 1.0 (control chars
// except tab/newline/CR) — command output routinely carries ANSI escapes,
// and encoding/xml would emit them verbatim, producing files CI parsers
// reject.
func sanitizeXML(s string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x09 || r == 0x0A || r == 0x0D || (r >= 0x20 && r != 0xFFFE && r != 0xFFFF) {
			return r
		}
		return -1
	}, s)
}

func renderJUnit(r *Report) ([]byte, error) {
	errorCount := 0
	for _, test := range r.Tests {
		if test.Status == StatusError {
			errorCount++
		}
	}
	suite := junitTestsuite{
		Name:     sanitizeXML(r.Suite),
		Tests:    len(r.Tests),
		Failures: r.Failed - errorCount,
		Errors:   errorCount,
		Skipped:  r.Skipped,
		Time:     fmt.Sprintf("%.3f", r.Duration.Seconds()),
	}
	// Invariant: Failed >= errorCount whenever the Report came from
	// FromRecords (errors count into Failed there); the clamp only guards
	// hand-built Reports with inconsistent totals
	if suite.Failures < 0 {
		suite.Failures = 0
	}
	for _, test := range r.Tests {
		testcase := junitTestcase{
			Name:      sanitizeXML(test.Name),
			Classname: sanitizeXML(test.Node),
			Time:      fmt.Sprintf("%.3f", test.Duration.Seconds()),
		}
		switch test.Status {
		case StatusFail:
			testcase.Failure = &junitMessage{
				Message: "checks failed",
				Body:    sanitizeXML(strings.Join(test.Failures, "\n")),
			}
		case StatusError:
			testcase.Error = &junitMessage{
				Message: "infrastructure error",
				Body:    sanitizeXML(strings.Join(test.Failures, "\n")),
			}
		case StatusSkip:
			testcase.Skipped = &junitMessage{Message: sanitizeXML(test.Reason)}
		}
		suite.Testcases = append(suite.Testcases, testcase)
	}
	data, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(data, '\n')...), nil
}
