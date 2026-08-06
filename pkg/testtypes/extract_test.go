package testtypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loomOutput = `{"summary": {"throughput_mbps": 12100.5, "streams": 8}}
p50=12us p99=45.2us
`

func TestExtractWithComparators(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("loom run", 0, loomOutput, "")

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command": "loom run",
		"extract": map[string]interface{}{
			"throughput_mbps": map[string]interface{}{"jsonpath": "$.summary.throughput_mbps"},
			"p99_us":          map[string]interface{}{"regex": `p99=([0-9.]+)us`},
			"streams":         map[string]interface{}{"jsonpath": "summary.streams"},
		},
		"evaluate": map[string]interface{}{
			"exit_code":       0,
			"throughput_mbps": map[string]interface{}{"gte": 11852, "within": 12476, "tolerance_pct": 5},
			"p99_us":          map[string]interface{}{"lte": 49},
			"streams":         map[string]interface{}{"eq": 8},
		},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

func TestExtractFailsOutsideBounds(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("loom run", 0, loomOutput, "")

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command": "loom run",
		"extract": map[string]interface{}{
			"p99_us": map[string]interface{}{"regex": `p99=([0-9.]+)us`},
		},
		"evaluate": map[string]interface{}{
			"p99_us": map[string]interface{}{"lte": 20},
		},
	})
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["p99_us"].Passed)
}

func TestExtractToleranceOutside(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("metric", 0, "90\n", "")

	// 90 is outside 100 ±5%
	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command": "metric",
		"extract": map[string]interface{}{
			"value": map[string]interface{}{"regex": `(\d+)`},
		},
		"evaluate": map[string]interface{}{
			"value": map[string]interface{}{"within": 100, "tolerance_pct": 5},
		},
	})
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["value"].Passed)

	// but inside 100 ±15 absolute
	node.SetResponse("metric2", 0, "90\n", "")
	ok, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command": "metric2",
		"extract": map[string]interface{}{
			"value": map[string]interface{}{"regex": `(\d+)`},
		},
		"evaluate": map[string]interface{}{
			"value": map[string]interface{}{"within": 100, "tolerance": 15},
		},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, ok))
}

func TestExtractMissingPatternFailsCheck(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("no-metrics", 0, "nothing here\n", "")

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command": "no-metrics",
		"extract": map[string]interface{}{
			"p99": map[string]interface{}{"regex": `p99=([0-9.]+)`},
		},
		"evaluate": map[string]interface{}{
			"p99": map[string]interface{}{"lte": 10},
		},
	})
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["p99"].Passed)
	assert.NoError(t, results["p99"].Err)
}

func TestExtractValidationErrors(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]struct {
		options  map[string]interface{}
		contains string
	}{
		"regex without group": {
			map[string]interface{}{"command": "x", "extract": map[string]interface{}{
				"v": map[string]interface{}{"regex": "p99"}}},
			"capture group",
		},
		"unknown extractor": {
			map[string]interface{}{"command": "x", "extract": map[string]interface{}{
				"v": map[string]interface{}{"xpath": "//x"}}},
			"unknown extractor",
		},
		"both jsonpath and regex": {
			map[string]interface{}{"command": "x", "extract": map[string]interface{}{
				"v": map[string]interface{}{"jsonpath": "a", "regex": "(a)"}}},
			"exactly one",
		},
		"unknown comparator": {
			map[string]interface{}{"command": "x",
				"extract":  map[string]interface{}{"v": map[string]interface{}{"regex": "(a)"}},
				"evaluate": map[string]interface{}{"v": map[string]interface{}{"approx": 5}}},
			"unknown comparator",
		},
		"within without tolerance": {
			map[string]interface{}{"command": "x",
				"extract":  map[string]interface{}{"v": map[string]interface{}{"regex": "(a)"}},
				"evaluate": map[string]interface{}{"v": map[string]interface{}{"within": 100}}},
			"requires tolerance",
		},
		"tolerance without within": {
			map[string]interface{}{"command": "x",
				"extract":  map[string]interface{}{"v": map[string]interface{}{"regex": "(a)"}},
				"evaluate": map[string]interface{}{"v": map[string]interface{}{"tolerance_pct": 5}}},
			"requires within",
		},
		"empty comparator map": {
			map[string]interface{}{"command": "x",
				"extract":  map[string]interface{}{"v": map[string]interface{}{"regex": "(a)"}},
				"evaluate": map[string]interface{}{"v": map[string]interface{}{}}},
			"no conditions",
		},
	}
	for name, tc := range cases {
		_, err := makeTest(t, node, TypeExecute, tc.options)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.contains, name)
	}
}

// Capture in one test, reference in later commands and skip conditions.
func TestCaptureAcrossTests(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("btrfs-show-id", 0, "266\n", "")
	node.SetResponse(`[ "300" -gt "266" ]`, 0, "", "")

	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name: "record id", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
			Options: map[string]interface{}{
				"command": "btrfs-show-id",
				"capture": "pre_rollback_root_id",
			},
		},
		{
			Name: "compare id", Node: config.NodeReference{"test-node"}, Type: TypeExecute, Order: 1,
			Options: map[string]interface{}{
				"command":  `[ "300" -gt "{{capture.pre_rollback_root_id}}" ]`,
				"evaluate": map[string]interface{}{"exit_code": 0},
			},
		},
	}
	tests, err := CreateTests(configs, nodes)
	require.NoError(t, err)
	require.Len(t, tests, 2)

	runTest(t, tests[0])
	allPassed(t, runTest(t, tests[1]))
}

func TestCaptureNamedExtracts(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("status-json", 0, `{"version": "1.2.3", "build": 42}`, "")
	node.SetResponse("echo 1.2.3-42", 0, "1.2.3-42\n", "")

	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name: "record versions", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
			Options: map[string]interface{}{
				"command": "status-json",
				"capture": map[string]interface{}{
					"version": map[string]interface{}{"jsonpath": "version"},
					"build":   map[string]interface{}{"jsonpath": "build"},
				},
			},
		},
		{
			Name: "use versions", Node: config.NodeReference{"test-node"}, Type: TypeExecute, Order: 1,
			Options: map[string]interface{}{
				"command":  "echo {{capture.version}}-{{capture.build}}",
				"evaluate": map[string]interface{}{"match": "1.2.3-42"},
			},
		},
	}
	tests, err := CreateTests(configs, nodes)
	require.NoError(t, err)
	runTest(t, tests[0])
	allPassed(t, runTest(t, tests[1]))
}

// Referencing a capture nothing recorded fails loudly instead of running a
// mangled command.
func TestDanglingCaptureReference(t *testing.T) {
	node := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name: "dangling", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
			Options: map[string]interface{}{
				"command": "echo {{capture.never_set}}",
			},
		},
	}
	tests, err := CreateTests(configs, nodes)
	require.NoError(t, err)

	_, err = tests[0].Run(formatters.NewMockTestCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "never_set")
}
