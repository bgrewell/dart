package testtypes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// existsCheck interprets a `test -e` exit code as file existence.
type existsCheck struct {
	expected bool
}

func (c *existsCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	exists := execResult.ExitCode == 0
	passed := exists == c.expected

	var details interface{} = fmt.Sprintf("exists=%v", exists)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("exists=%v", c.expected),
			Actual:   fmt.Sprintf("exists=%v", exists),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}

// hashCheck compares a checksum from *sum output against an expected hex
// digest. The algorithm is identified by digest length (md5=32, sha1=40,
// sha256=64), which is unambiguous when multiple sums share one output.
type hashCheck struct {
	algo     string
	hexLen   int
	expected string
}

var hexRe = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func (c *hashCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	for _, line := range strings.Split(string(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		candidate := fields[0]
		if len(candidate) != c.hexLen || !hexRe.MatchString(candidate) {
			continue
		}
		passed := strings.EqualFold(candidate, c.expected)
		var details interface{} = candidate
		if !passed {
			details = &results.ResultStringMatchFail{
				Expected: c.expected,
				Actual:   candidate,
			}
		}
		return &eval.EvaluateResult{Passed: passed, Details: details}
	}

	stderr, _ := execResult.StderrBytes()
	return &eval.EvaluateResult{
		Passed: false,
		Details: &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("%s digest in command output", c.algo),
			Actual:   strings.TrimSpace(string(stdout) + string(stderr)),
		},
	}
}

// packetLossCheck parses the "% packet loss" summary from ping output and
// passes when observed loss does not exceed the bound.
type packetLossCheck struct {
	max float64
}

var packetLossRe = regexp.MustCompile(`([0-9.]+)% packet loss`)

func (c *packetLossCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	m := packetLossRe.FindStringSubmatch(string(stdout))
	if m == nil {
		return &eval.EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: "ping output with a packet loss summary",
				Actual:   strings.TrimSpace(string(stdout)),
			},
		}
	}

	loss, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	passed := loss <= c.max
	var details interface{} = fmt.Sprintf("%.1f%% loss", loss)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("packet loss <= %v%%", c.max),
			Actual:   fmt.Sprintf("%.1f%%", loss),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}

// rttCheck parses the min/avg/max round-trip summary from ping output
// (iputils "rtt min/avg/max/mdev = a/b/c/d ms" and busybox
// "round-trip min/avg/max = a/b/c ms") and bounds one of the values:
// min is a lower bound, avg and max are upper bounds. Values are in ms.
type rttCheck struct {
	kind  string // "min", "avg", "max"
	bound float64
}

var rttRe = regexp.MustCompile(`min/avg/max[^=]*= *([0-9.]+)/([0-9.]+)/([0-9.]+)`)

func (c *rttCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	m := rttRe.FindStringSubmatch(string(stdout))
	if m == nil {
		return &eval.EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: "ping output with an rtt summary",
				Actual:   strings.TrimSpace(string(stdout)),
			},
		}
	}

	values := map[string]string{"min": m[1], "avg": m[2], "max": m[3]}
	observed, err := strconv.ParseFloat(values[c.kind], 64)
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	var passed bool
	var expected string
	if c.kind == "min" {
		passed = observed >= c.bound
		expected = fmt.Sprintf("rtt min >= %vms", c.bound)
	} else {
		passed = observed <= c.bound
		expected = fmt.Sprintf("rtt %s <= %vms", c.kind, c.bound)
	}

	var details interface{} = fmt.Sprintf("%.3fms", observed)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: expected,
			Actual:   fmt.Sprintf("%.3fms", observed),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}
