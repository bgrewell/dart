package helpers

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// WrapError records the caller's location, not WrapError's own.
func TestWrapErrorIncludesCallerLocation(t *testing.T) {
	err := WrapError("something broke")
	assert.Contains(t, err.Error(), "something broke")
	assert.Contains(t, err.Error(), "helpers_test.go")
}

func TestGetRandomId(t *testing.T) {
	seen := map[string]bool{}
	valid := regexp.MustCompile(`^[A-Za-z0-9]{8}$`)
	for i := 0; i < 100; i++ {
		id := GetRandomId()
		assert.Regexp(t, valid, id)
		seen[id] = true
	}
	assert.Greater(t, len(seen), 90, "ids should be effectively unique")
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'simple'", ShellQuote("simple"))
	assert.True(t, strings.HasPrefix(ShellQuote("it's"), "'it'"))
}
