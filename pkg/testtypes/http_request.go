package testtypes

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/probe"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &HTTPRequestTest{}

// HTTPRequestTest performs an HTTP request and applies evaluations to the
// response: the status code maps to the result's exit code and the body to
// its stdout, so status_code, contains, match, regex, json_path, and
// max_duration all work. The request is made from the test's node by
// default (from: host asks it from the machine running DART instead).
type HTTPRequestTest struct {
	BaseTest
	method  string
	url     string
	headers map[string]string
	timeout time.Duration
}

// newHTTPRequestTest parses url (required), method (default GET), headers
// (map), timeout seconds (default 30), and from (node|host, default node).
// Evaluate: status_code (default 200 when no evaluate block is given) plus
// the standard evaluators. Node-side requests are issued with curl, which
// the node must provide.
func newHTTPRequestTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	url, err := requiredString(base.name, opts, "url")
	if err != nil {
		return nil, err
	}
	method, present, err := optString(base.name, opts, "method")
	if err != nil {
		return nil, err
	}
	if !present || method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	timeoutSeconds, err := optFloat(base.name, opts, "timeout", 30)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, fmt.Errorf("timeout must be positive in test %q", base.name)
	}

	headers := map[string]string{}
	noteOption("headers")
	if raw, ok := opts["headers"]; ok {
		headerMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("headers must be a map in test %q (got %T)", base.name, raw)
		}
		for key, value := range headerMap {
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("header %q must be a string in test %q (got %T)", key, base.name, value)
			}
			headers[key] = s
		}
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	for name, value := range spec {
		if name == "status_code" {
			code, ok := coerceInt(value)
			if !ok {
				return nil, fmt.Errorf("status_code must be an integer in test %q (got %v)", base.name, value)
			}
			evaluations[name] = &eval.EvaluateExitCode{Expected: []int{code}}
			continue
		}
		evaluator, err := eval.New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}
	if len(evaluations) == 0 {
		evaluations["status_code"] = &eval.EvaluateExitCode{Expected: []int{http.StatusOK}}
	}

	from, err := parseVantage(base.name, opts)
	if err != nil {
		return nil, err
	}

	base.evaluations = evaluations
	if from == VantageNode {
		return &commandTest{
			BaseTest: base,
			// Built at run time so captured values are quoted after
			// substitution rather than injected into a quoted command
			build: func(resolve func(string) (string, error)) (string, error) {
				resolvedURL, err := resolve(url)
				if err != nil {
					return "", err
				}
				resolvedHeaders := make(map[string]string, len(headers))
				for name, value := range headers {
					resolvedHeaders[name], err = resolve(value)
					if err != nil {
						return "", err
					}
				}
				return probe.HTTPCommand(method, resolvedURL, resolvedHeaders, timeoutSeconds), nil
			},
			timeout: time.Duration((timeoutSeconds + 5) * float64(time.Second)),
			// The probe prints the body, then the status code on its own
			// line; the wrapper turns that into the same result shape the
			// host-side request produces
			transform: httpProbeResult,
		}, nil
	}

	return &HTTPRequestTest{
		BaseTest: base,
		method:   method,
		url:      url,
		headers:  headers,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// httpProbeResult reshapes the probe's output into the result the
// evaluators expect: the response body on stdout and the HTTP status as
// the exit code, matching the host-side request exactly.
func httpProbeResult(result *execution.ExecutionResult) (*execution.ExecutionResult, error) {
	stdout, err := result.StdoutBytes()
	if err != nil {
		return nil, err
	}
	stderr, _ := result.StderrBytes()

	if result.ExitCode == probe.MissingToolExitCode {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(stderr)))
	}

	body, status, err := probe.ParseHTTPOutput(string(stdout), string(stderr))
	if err != nil {
		return nil, err
	}

	return &execution.ExecutionResult{
		ExitCode: status,
		Stdout:   strings.NewReader(body),
		Stderr:   strings.NewReader(""),
		Duration: result.Duration,
	}, nil
}

func (t *HTTPRequestTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	return t.runProducer(t.perform, updater)
}

// perform executes the request and shapes the response as an execution
// result for the evaluators.
func (t *HTTPRequestTest) perform() (*execution.ExecutionResult, error) {
	client := &http.Client{Timeout: t.timeout}
	req, err := http.NewRequest(t.method, t.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &execution.ExecutionResult{
		ExitCode: resp.StatusCode,
		Stdout:   bytes.NewReader(body),
		Stderr:   strings.NewReader(""),
	}, nil
}
