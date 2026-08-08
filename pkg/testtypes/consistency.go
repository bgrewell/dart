package testtypes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/results"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &ConsistencyTest{}

// ConsistencyTest runs one command across several nodes and compares the
// results with each other — the shape config-drift and quorum questions
// need, which per-node tests cannot express. Unlike every other test type
// it is not expanded per node: its whole point is the comparison.
type ConsistencyTest struct {
	BaseTest
	command string
	nodes   []string
	timeout time.Duration
}

// NodeName reports every compared node, not just the first: the console,
// JUnit classname, and JSON report would otherwise name one node for a
// test that spans several.
func (t *ConsistencyTest) NodeName() string {
	return strings.Join(t.nodes, ",")
}

// nodeOutput is one node's result within a consistency run.
type nodeOutput struct {
	Node   string `json:"node"`
	Stdout string `json:"stdout"`
	// Digest is a hash of the raw output bytes. Comparison uses it rather
	// than Stdout because JSON marshaling replaces invalid UTF-8 with
	// U+FFFD, which would make genuinely different binary outputs collapse
	// into false agreement.
	Digest   string `json:"digest"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// consistencyReport is emitted as JSON on the synthesized result's stdout,
// so json_path and the standard evaluators work against it too.
type consistencyReport struct {
	Outputs []nodeOutput `json:"outputs"`
}

// newConsistencyTest parses command (required) and an optional node list
// (defaults to the test's `node:` list, which is not expanded for this
// type). Evaluate keys: all_equal (bool, the default check) requires every
// node's trimmed output to match; matching ({pattern, count}) requires
// exactly count nodes to match a regex — "exactly one leader" and
// "quorum of three" both fall out of it. Other keys fall through to the
// standard evaluators against the JSON report.
func newConsistencyTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	command, err := requiredString(base.name, opts, "command")
	if err != nil {
		return nil, err
	}

	nodeNames := base.nodeNames
	if listed, ok := opts["nodes"]; ok {
		names, ok := toStringList(listed)
		if !ok || len(names) == 0 {
			return nil, fmt.Errorf("nodes must be a non-empty list of node names in test %q", base.name)
		}
		// The narrowed list must come from the test's own node reference,
		// so the reported nodes always match the compared ones
		declared := map[string]bool{}
		for _, name := range base.nodeNames {
			declared[name] = true
		}
		for _, name := range names {
			if !declared[name] {
				return nil, fmt.Errorf("node %q in nodes: is not listed in the node: reference of test %q", name, base.name)
			}
		}
		nodeNames = names
	}
	// A repeated node would run twice and double-count in matching checks —
	// always a typo rather than an intent
	seen := map[string]bool{}
	for _, name := range nodeNames {
		if seen[name] {
			return nil, fmt.Errorf("node %q is listed more than once in consistency test %q", name, base.name)
		}
		seen[name] = true
		if _, ok := base.peerNodes[name]; !ok {
			return nil, fmt.Errorf("node %q not found (referenced in consistency test %q)", name, base.name)
		}
	}
	if len(nodeNames) < 2 {
		return nil, fmt.Errorf("consistency test %q needs at least two nodes to compare", base.name)
	}

	timeoutSeconds, err := optFloat(base.name, opts, "timeout", 0)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds < 0 {
		return nil, fmt.Errorf("timeout must be non-negative in test %q", base.name)
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	for name, value := range spec {
		switch name {
		case "all_equal":
			expected, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("all_equal must be a boolean in test %q", base.name)
			}
			evaluations[name] = &allEqualCheck{expected: expected}
		case "matching":
			settings, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("matching must be a {pattern, count} map in test %q (got %T)", base.name, value)
			}
			rawPattern, ok := settings["pattern"]
			if !ok {
				return nil, fmt.Errorf("matching requires a pattern in test %q", base.name)
			}
			pattern, ok := rawPattern.(string)
			if !ok {
				return nil, fmt.Errorf("matching pattern must be a string in test %q", base.name)
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("matching pattern in test %q is invalid: %w", base.name, err)
			}
			count := 1
			if rawCount, ok := settings["count"]; ok {
				count, ok = coerceInt(rawCount)
				if !ok || count < 0 {
					return nil, fmt.Errorf("matching count must be a non-negative integer in test %q", base.name)
				}
			}
			for key := range settings {
				if key != "pattern" && key != "count" {
					return nil, fmt.Errorf("unknown matching key %q in test %q", key, base.name)
				}
			}
			evaluations[name] = &matchingCountCheck{re: re, count: count}
		default:
			evaluator, err := eval.New(name, value)
			if err != nil {
				return nil, err
			}
			evaluations[name] = evaluator
		}
	}
	if len(evaluations) == 0 {
		evaluations["all_equal"] = &allEqualCheck{expected: true}
	}

	base.evaluations = evaluations
	return &ConsistencyTest{
		BaseTest: base,
		command:  command,
		nodes:    nodeNames,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

func (t *ConsistencyTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	command, err := t.interpolateCaptures(t.command)
	if err != nil {
		updater.Error()
		return nil, err
	}
	return t.runProducer(func() (*execution.ExecutionResult, error) {
		return t.gather(command)
	}, updater)
}

// gather runs the command on every node and shapes the per-node outputs as
// an execution result. A node that cannot run the command is recorded
// rather than aborting: "one node is unreachable" is itself a consistency
// finding, and the checks report which node failed.
func (t *ConsistencyTest) gather(command string) (*execution.ExecutionResult, error) {
	report := consistencyReport{Outputs: make([]nodeOutput, 0, len(t.nodes))}
	worstExit := 0

	for _, name := range t.nodes {
		node := t.peerNodes[name]
		out := nodeOutput{Node: name}

		result, err := ifaces.ExecuteWithTimeout(node, command, t.timeout)
		if err != nil {
			out.Error = err.Error()
			out.ExitCode = -1
			if worstExit < 1 {
				worstExit = 1
			}
		} else {
			stdout, readErr := result.StdoutBytes()
			if readErr != nil {
				out.Error = readErr.Error()
				if worstExit < 1 {
					worstExit = 1
				}
			}
			trimmed := strings.TrimRight(string(stdout), " \t\r\n")
			out.Stdout = trimmed
			digest := sha256.Sum256([]byte(trimmed))
			out.Digest = hex.EncodeToString(digest[:])
			out.ExitCode = result.ExitCode
			if result.ExitCode > worstExit {
				worstExit = result.ExitCode
			}
		}
		report.Outputs = append(report.Outputs, out)
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return &execution.ExecutionResult{
		ExitCode: worstExit,
		Stdout:   strings.NewReader(string(payload)),
		Stderr:   strings.NewReader(""),
	}, nil
}

// parseConsistencyReport reloads the per-node outputs from a result.
func parseConsistencyReport(execResult *execution.ExecutionResult) (*consistencyReport, *eval.EvaluateResult) {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return nil, &eval.EvaluateResult{Passed: false, Err: err}
	}
	var report consistencyReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		return nil, &eval.EvaluateResult{Passed: false, Err: err}
	}
	return &report, nil
}

// allEqualCheck requires every node's output to be identical. A node that
// errored can never be equal, so drift and unreachability both fail here.
type allEqualCheck struct {
	expected bool
}

func (c *allEqualCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	report, fail := parseConsistencyReport(execResult)
	if fail != nil {
		return fail
	}
	if len(report.Outputs) == 0 {
		return &eval.EvaluateResult{Passed: false, Details: "no nodes were compared"}
	}

	// A node that could not report is never a valid comparison — with
	// all_equal: false it would otherwise satisfy "nodes differ" and turn
	// an outage into a green test
	var failedNodes []string
	for _, out := range report.Outputs {
		if out.Error != "" {
			failedNodes = append(failedNodes, fmt.Sprintf("%s (%s)", out.Node, out.Error))
		}
	}
	if len(failedNodes) > 0 {
		sort.Strings(failedNodes)
		return &eval.EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: "every node to report",
				Actual:   "could not run on " + strings.Join(failedNodes, ", "),
			},
		}
	}

	// Group nodes by output digest so binary-different outputs cannot
	// collapse together; the readable text goes in the report
	groups := map[string][]string{}
	labels := map[string]string{}
	for _, out := range report.Outputs {
		groups[out.Digest] = append(groups[out.Digest], out.Node)
		labels[out.Digest] = out.Stdout
	}

	equal := len(groups) == 1
	passed := equal == c.expected
	if passed {
		if equal {
			return &eval.EvaluateResult{
				Passed:  true,
				Details: fmt.Sprintf("all %d nodes agree: %q", len(report.Outputs), report.Outputs[0].Stdout),
			}
		}
		return &eval.EvaluateResult{Passed: true, Details: describeGroups(groups, labels)}
	}

	expectedText := "all nodes to agree"
	if !c.expected {
		expectedText = "nodes to differ"
	}
	return &eval.EvaluateResult{
		Passed: false,
		Details: &results.ResultStringMatchFail{
			Expected: expectedText,
			Actual:   describeGroups(groups, labels),
		},
	}
}

// describeGroups renders output groups deterministically for reporting,
// keyed by digest but labeled with the readable output.
func describeGroups(groups map[string][]string, labels map[string]string) string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		nodes := groups[key]
		sort.Strings(nodes)
		parts = append(parts, fmt.Sprintf("%s => %q", strings.Join(nodes, ","), labels[key]))
	}
	return strings.Join(parts, " | ")
}

// matchingCountCheck requires exactly count nodes to match a pattern —
// "exactly one leader", "two replicas in sync".
type matchingCountCheck struct {
	re    *regexp.Regexp
	count int
}

func (c *matchingCountCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	report, fail := parseConsistencyReport(execResult)
	if fail != nil {
		return fail
	}

	var matched []string
	for _, out := range report.Outputs {
		if out.Error == "" && c.re.MatchString(out.Stdout) {
			matched = append(matched, out.Node)
		}
	}
	sort.Strings(matched)

	passed := len(matched) == c.count
	var details interface{} = fmt.Sprintf("%d matching: %s", len(matched), strings.Join(matched, ","))
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("exactly %d node(s) matching /%s/", c.count, c.re.String()),
			Actual:   fmt.Sprintf("%d matching (%s)", len(matched), strings.Join(matched, ",")),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}
