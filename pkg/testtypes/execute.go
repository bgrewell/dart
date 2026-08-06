package testtypes

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &ExecutionTest{}

type ExecuteTestConfig struct {
	Command  string                 `yaml:"command"`
	Evaluate map[string]interface{} `yaml:"evaluate"`
}

func NewExecuteTest(base BaseTest, opts *map[string]interface{}) (test ifaces.Test, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var testCfg ExecuteTestConfig
	err = json.Unmarshal(jsonData, &testCfg)
	if err != nil {
		return nil, err
	}

	evaluations, err := eval.Parse(testCfg.Evaluate)
	if err != nil {
		return nil, err
	}

	base.evaluations = evaluations

	test = &ExecutionTest{
		BaseTest: base,
		execute:  testCfg.Command,
	}
	return test, nil
}

type ExecutionTest struct {
	BaseTest
	execute string
}

func (t *ExecutionTest) Name() string {
	return t.name
}

func (t *ExecutionTest) NodeName() string {
	return t.nodeName
}

func (t *ExecutionTest) Run(updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error) {

	// Run pre-execute commands; a failure here fails the test before it runs
	updater.Update("preparing")
	for _, cmd := range t.setup {
		_, err = t.node.Execute(cmd)
		if err != nil {
			updater.Error()
			return nil, err
		}

	}

	// Run the test command
	updater.Update("running")
	start := time.Now()
	testResult, testErr := t.node.Execute(t.execute)
	if testResult != nil {
		testResult.Duration = time.Since(start)
	}

	// Post-execute commands always run, even after a test failure, since
	// they are part of cleanup
	updater.Update("cleanup")
	var teardownErr error
	for _, cmd := range t.teardown {
		if _, cmdErr := t.node.Execute(cmd); cmdErr != nil {
			teardownErr = cmdErr
			break
		}
	}

	if testErr != nil {
		updater.Error()
		return nil, testErr
	}

	// Drain both streams up front so buffered output is captured even when
	// no output-based evaluations are configured
	_, _ = testResult.StdoutBytes()
	_, _ = testResult.StderrBytes()

	// Evaluations run in sorted-name order so reported results are
	// deterministic
	names := make([]string, 0, len(t.evaluations))
	for name := range t.evaluations {
		names = append(names, name)
	}
	sort.Strings(names)

	results = make(map[string]*eval.EvaluateResult, len(names))
	passed := make([]bool, 0, len(names))
	for _, name := range names {
		result := t.evaluations[name].Verify(testResult)
		passed = append(passed, result.Passed)
		results[name] = result
	}

	// A teardown failure is surfaced after evaluation so the test outcome
	// isn't lost, but it still aborts the run since the system state is
	// unknown at that point
	if teardownErr != nil {
		updater.Error()
		return results, teardownErr
	}

	updater.Complete(passed)
	return results, nil
}
