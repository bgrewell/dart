package steptypes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
)

// TestHTTPRequestStep verifies HTTP response handling.
func TestHTTPRequestStep(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	step := &HTTPRequestStep{
		BaseStep:       BaseStep{title: "HTTP Test"},
		method:         "GET",
		url:            server.URL,
		expectedStatus: 200,
		expectedBody:   "success",
		timeout:        5 * time.Second,
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Validate response
	assert.NoError(t, err)
}
