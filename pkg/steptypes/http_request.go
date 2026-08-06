package steptypes

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &HTTPRequestStep{}

// HTTPRequestStep performs an HTTP request and checks the response.
// Note: the request is made from the host running DART, not from the
// step's node — it verifies reachability from the controller's viewpoint.
type HTTPRequestStep struct {
	BaseStep
	method         string
	url            string
	expectedStatus int
	expectedBody   string
	timeout        time.Duration
}

// newHTTPRequestStep parses url (required), method (default GET),
// expected_status (default 200), expected_body, and timeout seconds
// (default 30).
func newHTTPRequestStep(c *config.StepConfig, _ ifaces.Node) (ifaces.Step, error) {
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

	return &HTTPRequestStep{
		BaseStep:       baseFor(c),
		method:         method,
		url:            url,
		expectedStatus: expectedStatus,
		expectedBody:   expectedBody,
		timeout:        time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// Run executes the HTTP request and verifies the response.
func (s *HTTPRequestStep) Run(updater formatters.TaskCompleter) error {
	client := &http.Client{Timeout: s.timeout}
	req, err := http.NewRequest(s.method, s.url, nil)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		updater.Error()
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != s.expectedStatus {
		updater.Error()
		return fmt.Errorf("unexpected status code: got %d, expected %d", resp.StatusCode, s.expectedStatus)
	}

	if s.expectedBody != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			updater.Error()
			return fmt.Errorf("failed to read response body: %w", err)
		}
		if !strings.Contains(string(body), s.expectedBody) {
			updater.Error()
			return fmt.Errorf("response validation failed: expected content missing")
		}
	}

	updater.Complete()
	return nil
}
