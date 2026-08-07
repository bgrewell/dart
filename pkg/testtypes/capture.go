package testtypes

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// captureStore holds values captured by earlier tests for interpolation
// into later ones. One store is shared by all tests of a suite; on
// repeated iterations values are overwritten by each run.
type captureStore struct {
	mu     sync.Mutex
	values map[string]string
}

func newCaptureStore() *captureStore {
	return &captureStore{values: make(map[string]string)}
}

func (s *captureStore) set(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = value
}

var captureRefRe = regexp.MustCompile(`\{\{\s*capture\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// interpolate replaces {{capture.name}} references in text. Referencing a
// value that no earlier test captured is an error — a dangling reference
// must fail loudly, not run a mangled command.
func (s *captureStore) interpolate(text string) (string, error) {
	var missing []string
	replaced := captureRefRe.ReplaceAllStringFunc(text, func(ref string) string {
		name := captureRefRe.FindStringSubmatch(ref)[1]
		s.mu.Lock()
		value, ok := s.values[name]
		s.mu.Unlock()
		if !ok {
			missing = append(missing, name)
			return ref
		}
		return value
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("no captured value named %s — the capturing test must run earlier in the suite and must not be skipped or excluded by --only/--skip tag filters", strings.Join(missing, ", "))
	}
	return replaced, nil
}

// interpolateCaptures resolves capture references in text against the
// test's store. Text without references passes through untouched, so
// tests constructed without a store (e.g. directly in unit tests) work
// unless they actually reference captures.
func (t *BaseTest) interpolateCaptures(text string) (string, error) {
	if !captureRefRe.MatchString(text) {
		return text, nil
	}
	if t.captures == nil {
		return "", fmt.Errorf("test %q references captured values but no capture store exists", t.name)
	}
	return t.captures.interpolate(text)
}

// captureSpec names a value to record from a test's stdout. A nil
// extractor captures the whole trimmed stdout.
type captureSpec struct {
	name string
	ext  extractor
}

// parseCaptureSpecs accepts `capture: name` (whole trimmed stdout) or
// `capture: {name: {jsonpath|regex: ...}, ...}` for extracted values.
func parseCaptureSpecs(testName string, opts map[string]interface{}) ([]captureSpec, error) {
	raw, ok := opts["capture"]
	if !ok {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil, fmt.Errorf("capture name must be non-empty in test %q", testName)
		}
		if !captureNameRe.MatchString(v) {
			return nil, fmt.Errorf("capture name %q in test %q must match %s", v, testName, captureNameRe.String())
		}
		return []captureSpec{{name: v}}, nil
	case map[string]interface{}:
		specs := make([]captureSpec, 0, len(v))
		for name, extRaw := range v {
			if !captureNameRe.MatchString(name) {
				return nil, fmt.Errorf("capture name %q in test %q must match %s", name, testName, captureNameRe.String())
			}
			ext, err := parseExtractor(testName, name, extRaw)
			if err != nil {
				return nil, err
			}
			specs = append(specs, captureSpec{name: name, ext: ext})
		}
		return specs, nil
	default:
		return nil, fmt.Errorf("capture in test %q must be a name or a map of name to extractor (got %T)", testName, raw)
	}
}

var captureNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
