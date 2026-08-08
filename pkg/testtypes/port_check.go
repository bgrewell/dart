package testtypes

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/helpers"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &PortCheckTest{}

// PortCheckTest attempts a TCP connection and reports the port as "open"
// or "closed". The probe runs on the test's node by default; from: host
// attempts it from the machine running DART instead.
type PortCheckTest struct {
	BaseTest
	host    string
	port    int
	timeout time.Duration
}

// newPortCheckTest parses host and port (required), timeout seconds
// (default 5), and from (node|host, default node). Probing from the node
// is what answers firewall and ACL questions ("can node A reach B:5432,
// is node C blocked?"); from: host asks whether the controller can reach
// the address instead. Evaluate: status ("open" or "closed", default "open");
// other keys fall through to the standard evaluators (the observed status
// is the result's stdout, closed also sets a non-zero exit code).
func newPortCheckTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	host, err := requiredString(base.name, opts, "host")
	if err != nil {
		return nil, err
	}
	from, err := parseVantage(base.name, opts)
	if err != nil {
		return nil, err
	}
	port, err := optInt(base.name, opts, "port", 0)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535 in test %q", base.name)
	}
	timeoutSeconds, err := optFloat(base.name, opts, "timeout", 5)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, fmt.Errorf("timeout must be positive in test %q", base.name)
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	status := "open"
	for name, value := range spec {
		if name == "status" {
			s, ok := value.(string)
			if !ok || (s != "open" && s != "closed") {
				return nil, fmt.Errorf("status must be \"open\" or \"closed\" in test %q (got %v)", base.name, value)
			}
			status = s
			continue
		}
		evaluator, err := eval.New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}
	evaluations["status"] = &eval.EvaluateMatch{Expected: status, Trim: true}

	base.evaluations = evaluations

	if from == VantageNode {
		return &commandTest{
			BaseTest: base,
			// Built at run time so a captured host is quoted after
			// substitution rather than injected into a quoted command
			build: func(resolve func(string) (string, error)) (string, error) {
				resolvedHost, err := resolve(host)
				if err != nil {
					return "", err
				}
				return nodePortProbe(resolvedHost, port, timeoutSeconds), nil
			},
			// DART bounds the probe itself, so a node without a working
			// `timeout` binary still cannot hang the suite
			timeout: time.Duration((timeoutSeconds + 5) * float64(time.Second)),
		}, nil
	}

	return &PortCheckTest{
		BaseTest: base,
		host:     host,
		port:     port,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

func (t *PortCheckTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	return t.runProducer(t.probe, updater)
}

// probe dials the port and shapes the observed state as an execution
// result for the evaluators.
func (t *PortCheckTest) probe() (*execution.ExecutionResult, error) {
	address := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	conn, err := net.DialTimeout("tcp", address, t.timeout)

	status := "open"
	exitCode := 0
	if err != nil {
		status = "closed"
		exitCode = 1
	} else {
		conn.Close()
	}

	return &execution.ExecutionResult{
		ExitCode: exitCode,
		Stdout:   strings.NewReader(status),
		Stderr:   strings.NewReader(""),
	}, nil
}

// nodePortProbe builds the shell probe for `from: node`. Design notes,
// each guarding a real failure mode found in review:
//
//   - The host never reaches an inner shell as text: bash receives it as a
//     positional argument, so quotes or semicolons in a host (which can
//     arrive from a fact, var, or capture) become part of the hostname and
//     fail the connection rather than executing.
//   - `nc` is only used after proving the local build accepts -z: busybox
//     builds without FEATURE_NC_EXTRA reject it, which would otherwise make
//     every port report closed.
//   - When no usable method exists the probe prints "unsupported" instead
//     of guessing, so the status check fails loudly rather than silently
//     reporting a wrong answer.
func nodePortProbe(host string, port int, timeoutSeconds float64) string {
	timeoutSecs := int(math.Ceil(timeoutSeconds))
	if timeoutSecs < 1 {
		timeoutSecs = 1
	}

	return fmt.Sprintf(`dart_h=%s; dart_p=%d; dart_t=%d
if command -v timeout >/dev/null 2>&1; then dart_to="timeout $dart_t"; else dart_to=""; fi
if command -v bash >/dev/null 2>&1; then
  if $dart_to bash -c 'exec 3<>/dev/tcp/"$0"/"$1"' "$dart_h" "$dart_p" >/dev/null 2>&1; then echo open; else echo closed; fi
elif command -v nc >/dev/null 2>&1 && ! nc -z -w "$dart_t" "$dart_h" "$dart_p" 2>&1 | grep -qiE 'invalid option|unrecognized option|bad option|illegal option'; then
  if nc -z -w "$dart_t" "$dart_h" "$dart_p" >/dev/null 2>&1; then echo open; else echo closed; fi
else
  echo unsupported
fi`, helpers.ShellQuote(host), port, timeoutSecs)
}
