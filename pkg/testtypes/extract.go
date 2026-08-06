package testtypes

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// extractor pulls a named value out of a command's stdout.
type extractor interface {
	extract(stdout string) (string, error)
	describe() string
}

// jsonPathExtractor resolves a dot-path (optionally "$."-prefixed) in JSON
// output.
type jsonPathExtractor struct {
	path string
}

func (e *jsonPathExtractor) extract(stdout string) (string, error) {
	value, err := eval.ExtractJSONPath(stdout, e.path)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(value), nil
}

func (e *jsonPathExtractor) describe() string {
	return fmt.Sprintf("jsonpath %q", e.path)
}

// regexExtractor returns capture group 1 of the first match.
type regexExtractor struct {
	re *regexp.Regexp
}

func (e *regexExtractor) extract(stdout string) (string, error) {
	m := e.re.FindStringSubmatch(stdout)
	if m == nil {
		return "", fmt.Errorf("pattern /%s/ did not match the output", e.re.String())
	}
	if len(m) < 2 {
		return "", fmt.Errorf("pattern /%s/ has no capture group", e.re.String())
	}
	return m[1], nil
}

func (e *regexExtractor) describe() string {
	return fmt.Sprintf("regex /%s/", e.re.String())
}

// parseExtractor builds an extractor from a {jsonpath: ...} or
// {regex: ...} spec. Regex patterns must contain a capture group.
func parseExtractor(testName, valueName string, raw interface{}) (extractor, error) {
	spec, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("extract %q in test %q must be a {jsonpath} or {regex} map (got %T)", valueName, testName, raw)
	}
	if len(spec) != 1 {
		return nil, fmt.Errorf("extract %q in test %q must have exactly one of jsonpath/regex", valueName, testName)
	}

	if raw, ok := spec["jsonpath"]; ok {
		path, ok := raw.(string)
		if !ok || path == "" {
			return nil, fmt.Errorf("jsonpath for extract %q in test %q must be a non-empty string", valueName, testName)
		}
		if err := eval.ValidateJSONPath(strings.TrimPrefix(strings.TrimPrefix(path, "$."), "$")); err != nil {
			return nil, fmt.Errorf("extract %q in test %q: %w", valueName, testName, err)
		}
		return &jsonPathExtractor{path: path}, nil
	}

	if raw, ok := spec["regex"]; ok {
		pattern, ok := raw.(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("regex for extract %q in test %q must be a non-empty string", valueName, testName)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("extract %q in test %q: invalid pattern: %w", valueName, testName, err)
		}
		if re.NumSubexp() < 1 {
			return nil, fmt.Errorf("extract %q in test %q: pattern must have a capture group", valueName, testName)
		}
		return &regexExtractor{re: re}, nil
	}

	for key := range spec {
		return nil, fmt.Errorf("unknown extractor %q for %q in test %q (use jsonpath or regex)", key, valueName, testName)
	}
	return nil, nil
}

// comparator is one condition applied to an extracted value.
type comparator struct {
	op        string
	value     interface{} // numeric bound, or raw expected for eq/ne
	tolerance float64     // absolute tolerance for within
}

// parseComparators builds the condition set from an evaluate entry like
// {gte: 100, lte: 200} or {within: 12476, tolerance_pct: 5}. Supported
// ops: gt/gte/lt/lte (ge/le accepted as aliases), eq, ne, within (with
// tolerance_pct or tolerance).
func parseComparators(testName, valueName string, raw interface{}) ([]comparator, error) {
	spec, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("evaluate %q in test %q must be a comparator map like {gte: 100} (got %T)", valueName, testName, raw)
	}

	var within *comparator
	var tolerancePct, toleranceAbs *float64
	var comparators []comparator

	// Deterministic processing order for stable error messages
	keys := make([]string, 0, len(spec))
	for key := range spec {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := spec[key]
		switch key {
		case "gt", "gte", "lt", "lte", "ge", "le":
			bound, ok := coerceFloat(value)
			if !ok {
				return nil, fmt.Errorf("%s for %q in test %q must be a number (got %v)", key, valueName, testName, value)
			}
			op := key
			switch op {
			case "ge":
				op = "gte"
			case "le":
				op = "lte"
			}
			comparators = append(comparators, comparator{op: op, value: bound})
		case "eq", "ne":
			comparators = append(comparators, comparator{op: key, value: value})
		case "within":
			base, ok := coerceFloat(value)
			if !ok {
				return nil, fmt.Errorf("within for %q in test %q must be a number (got %v)", valueName, testName, value)
			}
			within = &comparator{op: "within", value: base}
		case "tolerance_pct":
			pct, ok := coerceFloat(value)
			if !ok || pct < 0 {
				return nil, fmt.Errorf("tolerance_pct for %q in test %q must be a non-negative number", valueName, testName)
			}
			tolerancePct = &pct
		case "tolerance":
			abs, ok := coerceFloat(value)
			if !ok || abs < 0 {
				return nil, fmt.Errorf("tolerance for %q in test %q must be a non-negative number", valueName, testName)
			}
			toleranceAbs = &abs
		default:
			return nil, fmt.Errorf("unknown comparator %q for %q in test %q", key, valueName, testName)
		}
	}

	if within != nil {
		switch {
		case tolerancePct != nil && toleranceAbs != nil:
			return nil, fmt.Errorf("within for %q in test %q takes tolerance_pct or tolerance, not both", valueName, testName)
		case tolerancePct != nil:
			base, _ := coerceFloat(within.value)
			within.tolerance = base * *tolerancePct / 100
		case toleranceAbs != nil:
			within.tolerance = *toleranceAbs
		default:
			return nil, fmt.Errorf("within for %q in test %q requires tolerance_pct or tolerance", valueName, testName)
		}
		if within.tolerance < 0 {
			within.tolerance = -within.tolerance
		}
		comparators = append(comparators, *within)
	} else if tolerancePct != nil || toleranceAbs != nil {
		return nil, fmt.Errorf("tolerance for %q in test %q requires within", valueName, testName)
	}

	if len(comparators) == 0 {
		return nil, fmt.Errorf("evaluate %q in test %q has no conditions", valueName, testName)
	}
	return comparators, nil
}

var comparatorSymbols = map[string]string{
	"gt": ">", "gte": ">=", "lt": "<", "lte": "<=", "eq": "==", "ne": "!=",
}

// valueCheck extracts a named value from the output and applies its
// comparator conditions; all must hold.
type valueCheck struct {
	name        string
	ext         extractor
	comparators []comparator
}

func (c *valueCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return &eval.EvaluateResult{Passed: false, Err: err}
	}

	raw, err := c.ext.extract(string(stdout))
	if err != nil {
		return &eval.EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: fmt.Sprintf("%s extracted via %s", c.name, c.ext.describe()),
				Actual:   err.Error(),
			},
		}
	}

	var failures []string
	for _, cmp := range c.comparators {
		expected, ok := c.check(cmp, raw)
		if !ok {
			failures = append(failures, expected)
		}
	}

	if len(failures) > 0 {
		return &eval.EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: fmt.Sprintf("%s %s", c.name, strings.Join(failures, " and ")),
				Actual:   raw,
			},
		}
	}
	return &eval.EvaluateResult{Passed: true, Details: fmt.Sprintf("%s=%s", c.name, raw)}
}

// check applies one comparator, returning its description and whether it
// held. eq/ne compare numerically when both sides parse as numbers, by
// string otherwise; the ordered and within comparators require numbers.
func (c *valueCheck) check(cmp comparator, raw string) (description string, ok bool) {
	switch cmp.op {
	case "eq", "ne":
		expected := fmt.Sprint(cmp.value)
		equal := false
		if observed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			if bound, numOK := coerceFloat(cmp.value); numOK {
				equal = observed == bound
			} else {
				equal = raw == expected
			}
		} else {
			equal = raw == expected
		}
		if cmp.op == "ne" {
			return fmt.Sprintf("!= %v", cmp.value), !equal
		}
		return fmt.Sprintf("== %v", cmp.value), equal
	case "within":
		base, _ := coerceFloat(cmp.value)
		description = fmt.Sprintf("within %v ±%v", base, cmp.tolerance)
		observed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return description + " (numeric)", false
		}
		diff := observed - base
		if diff < 0 {
			diff = -diff
		}
		return description, diff <= cmp.tolerance
	default:
		bound, _ := coerceFloat(cmp.value)
		description = fmt.Sprintf("%s %v", comparatorSymbols[cmp.op], bound)
		observed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return description + " (numeric)", false
		}
		switch cmp.op {
		case "gt":
			return description, observed > bound
		case "gte":
			return description, observed >= bound
		case "lt":
			return description, observed < bound
		case "lte":
			return description, observed <= bound
		}
	}
	return "unknown comparator", false
}
