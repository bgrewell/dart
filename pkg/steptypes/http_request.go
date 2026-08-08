package steptypes

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/probe"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &HTTPRequestStep{}

// HTTPRequestStep performs an HTTP request and checks the response. The
// request is made from the step's node by default, so it reflects what that
// node can reach; from: host asks it from the machine running DART instead.
// Node-side requests are issued with curl, which the node must provide.
type HTTPRequestStep struct {
	BaseStep
	node           ifaces.Node
	from           Vantage
	method         string
	url            string
	headers        map[string]string
	expectedStatus int
	expectedBody   string
	timeout        time.Duration
}

// newHTTPRequestStep parses url (required), method (default GET), headers
// (map), expected_status (default 200), expected_body, timeout seconds
// (default 30), and from (node|host, default node).
func newHTTPRequestStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	url, err := requiredString(c, "url", "url is required")
	if err != nil {
		return nil, err
	}
	method, present, err := optString(c, "method")
	if err != nil {
		return nil, err
	}
	if !present || method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	expectedStatus, err := optInt(c, "expected_status", http.StatusOK)
	if err != nil {
		return nil, err
	}
	expectedBody, _, err := optString(c, "expected_body")
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := optFloat(c, "timeout", 30)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, optionError(c, "timeout must be positive in step %q", c.Name)
	}
	from, err := parseVantage(c)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	if raw, ok := c.Step.Options["headers"]; ok {
		headerMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, optionError(c, "headers must be a map in step %q (got %T)", c.Name, raw)
		}
		for key, value := range headerMap {
			s, ok := value.(string)
			if !ok {
				return nil, optionError(c, "header %q must be a string in step %q (got %T)", key, c.Name, value)
			}
			headers[key] = s
		}
	}

	return &HTTPRequestStep{
		BaseStep:       baseFor(c),
		node:           node,
		from:           from,
		method:         method,
		url:            url,
		headers:        headers,
		expectedStatus: expectedStatus,
		expectedBody:   expectedBody,
		timeout:        time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// Run executes the HTTP request and verifies the response.
func (s *HTTPRequestStep) Run(updater formatters.TaskCompleter) error {
	status, body, err := s.request()
	if err != nil {
		updater.Error()
		return err
	}

	if status != s.expectedStatus {
		updater.Error()
		return fmt.Errorf("unexpected status code: got %d, expected %d", status, s.expectedStatus)
	}

	if s.expectedBody != "" && !strings.Contains(body, s.expectedBody) {
		updater.Error()
		return fmt.Errorf("response validation failed: expected content missing")
	}

	updater.Complete()
	return nil
}

// request issues the request from the configured vantage and returns the
// status code and body, identically shaped either way.
func (s *HTTPRequestStep) request() (int, string, error) {
	if s.from == VantageNode {
		return s.requestFromNode()
	}
	return s.requestFromHost()
}

func (s *HTTPRequestStep) requestFromNode() (int, string, error) {
	command := probe.HTTPCommand(s.method, s.url, s.headers, s.timeout.Seconds())
	// The node's own timeout bounds curl; the outer bound covers a hung
	// transport that never returns curl's output
	result, err := ifaces.ExecuteWithTimeout(s.node, command, s.timeout+5*time.Second)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	stdout, err := result.StdoutBytes()
	if err != nil {
		return 0, "", fmt.Errorf("failed to read response: %w", err)
	}
	stderr, _ := result.StderrBytes()
	if result.ExitCode == probe.MissingToolExitCode {
		return 0, "", fmt.Errorf("%s", strings.TrimSpace(string(stderr)))
	}

	body, status, err := probe.ParseHTTPOutput(string(stdout), string(stderr))
	if err != nil {
		return 0, "", err
	}
	return status, body, nil
}

func (s *HTTPRequestStep) requestFromHost() (int, string, error) {
	client := &http.Client{Timeout: s.timeout}
	req, err := http.NewRequest(s.method, s.url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}
	for key, value := range s.headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// The body is only needed for expected_body, but reading it fully lets
	// the connection return to the pool
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, string(body), nil
}
