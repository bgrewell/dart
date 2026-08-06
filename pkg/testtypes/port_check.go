package testtypes

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &PortCheckTest{}

// PortCheckTest attempts a TCP connection and reports the port as "open"
// or "closed". Note: the connection is attempted from the host running
// DART, not from the test's node.
type PortCheckTest struct {
	BaseTest
	host    string
	port    int
	timeout time.Duration
}

// newPortCheckTest parses host and port (required) and timeout seconds
// (default 5). Evaluate: status ("open" or "closed", default "open");
// other keys fall through to the standard evaluators (the observed status
// is the result's stdout, closed also sets a non-zero exit code).
func newPortCheckTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	host, err := requiredString(base.name, opts, "host")
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
