package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/pkg/ifaces"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
)

var _ ifaces.Step = &HTTPRequestStep{}

// HTTPRequestStep performs an HTTP request and checks the response.
type HTTPRequestStep struct {
	BaseStep
	method         string
	url            string
	expectedStatus int
	expectedBody   string
	timeout        time.Duration
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
