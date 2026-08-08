package eval

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateJSONPath parses stdout as JSON and compares the value at a
// dot-path (e.g. "result.items[0].name") against an expected value.
// Comparison is loose: numbers compare numerically regardless of int/float
// representation, everything else by string form.
type EvaluateJSONPath struct {
	Path     string
	Expected interface{}
}

// trimJSONPathPrefix accepts the JSONPath-style "$." / "$" prefix, so a
// path written for extract: also works here.
func trimJSONPathPrefix(path string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, "$."), "$")
}

// newJSONPath accepts a {path, equals} map. The path is validated at
// config-load time.
func newJSONPath(value interface{}) (Evaluate, error) {
	spec, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a {path, equals} map, got %T", value)
	}
	rawPath, ok := spec["path"]
	if !ok {
		return nil, fmt.Errorf("missing required key %q", "path")
	}
	path, err := asString(rawPath)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", "path", err)
	}
	expected, ok := spec["equals"]
	if !ok {
		return nil, fmt.Errorf("missing required key %q", "equals")
	}
	for key := range spec {
		if key != "path" && key != "equals" {
			return nil, fmt.Errorf("unknown key %q", key)
		}
	}
	path = trimJSONPathPrefix(path)
	if _, err := parseJSONPath(path); err != nil {
		return nil, err
	}
	return &EvaluateJSONPath{Path: path, Expected: expected}, nil
}

// Verify is a method that verifies the JSON value at the path
func (j *EvaluateJSONPath) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := streamStdout.read(execResult)
	if err != nil {
		return errResult(err)
	}

	var doc interface{}
	if err := json.Unmarshal([]byte(actual), &doc); err != nil {
		return &EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: "valid JSON output",
				Actual:   strings.TrimSpace(actual),
			},
			Err: nil,
		}
	}

	value, err := resolveJSONPath(doc, j.Path)
	if err != nil {
		return &EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: fmt.Sprintf("value at %q", j.Path),
				Actual:   err.Error(),
			},
			Err: nil,
		}
	}

	passed := looselyEqual(value, j.Expected)
	var details interface{} = fmt.Sprint(value)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprint(j.Expected),
			Actual:   fmt.Sprint(value),
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}

// ExtractJSONPath decodes the first JSON value in jsonText (tool output
// often mixes a JSON document with plain-text lines after it) and returns
// the value at the dot-path (e.g. "summary.throughput_mbps",
// "items[0].name"). A leading "$." or "$" (JSONPath style) is accepted
// and ignored.
func ExtractJSONPath(jsonText, path string) (interface{}, error) {
	path = trimJSONPathPrefix(path)
	var doc interface{}
	decoder := json.NewDecoder(strings.NewReader(jsonText))
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("output is not valid JSON: %w", err)
	}
	return resolveJSONPath(doc, path)
}

// ValidateJSONPath reports whether path parses, for config-time validation.
func ValidateJSONPath(path string) error {
	path = trimJSONPathPrefix(path)
	_, err := parseJSONPath(path)
	return err
}

// jsonPathSegment is one dot-separated element of a path: an object key
// (possibly empty for a root array) followed by zero or more array indices.
type jsonPathSegment struct {
	key     string
	indices []int
}

func parseJSONPath(path string) ([]jsonPathSegment, error) {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil, fmt.Errorf("empty json path")
	}
	parts := strings.Split(path, ".")
	segments := make([]jsonPathSegment, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty segment in json path %q", path)
		}
		key := part
		var indices []int
		for strings.HasSuffix(key, "]") {
			open := strings.LastIndex(key, "[")
			if open < 0 {
				return nil, fmt.Errorf("unbalanced brackets in json path segment %q", part)
			}
			idx, err := strconv.Atoi(key[open+1 : len(key)-1])
			if err != nil || idx < 0 {
				return nil, fmt.Errorf("invalid array index in json path segment %q", part)
			}
			indices = append([]int{idx}, indices...)
			key = key[:open]
		}
		segments = append(segments, jsonPathSegment{key: key, indices: indices})
	}
	return segments, nil
}

func resolveJSONPath(doc interface{}, path string) (interface{}, error) {
	segments, err := parseJSONPath(path)
	if err != nil {
		return nil, err
	}
	cur := doc
	for _, seg := range segments {
		if seg.key != "" {
			m, ok := cur.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot access key %q: parent is not an object", seg.key)
			}
			cur, ok = m[seg.key]
			if !ok {
				return nil, fmt.Errorf("key %q not found", seg.key)
			}
		}
		for _, idx := range seg.indices {
			arr, ok := cur.([]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot index into non-array at %q", seg.key)
			}
			if idx >= len(arr) {
				return nil, fmt.Errorf("index %d out of range at %q (length %d)", idx, seg.key, len(arr))
			}
			cur = arr[idx]
		}
	}
	return cur, nil
}

func looselyEqual(a, b interface{}) bool {
	if af, aok := asNumber(a); aok {
		if bf, bok := asNumber(b); bok {
			return af == bf
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func asNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
