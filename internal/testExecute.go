package internal

import (
	"encoding/json"
	"fmt"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
)

type ExecuteTestConfig struct {
	Command  string                 `yaml:"command"`
	Evaluate map[string]interface{} `yaml:"evaluate"`
}

func NewExecuteTest(base BaseTest, opts *map[string]interface{}) (test Test, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var testCfg ExecuteTestConfig
	err = json.Unmarshal(jsonData, &testCfg)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate)
	for k, v := range testCfg.Evaluate {
		switch k {
		case "exit_code":
			switch v := v.(type) {
			case int:
				chk := &eval.EvaluateExitCode{
					Expected: v,
				}
				evaluations[k] = chk
			case float64:
				chk := &eval.EvaluateExitCode{
					Expected: int(v),
				}
				evaluations[k] = chk
			default:
				return nil, fmt.Errorf("invalid type for exit_code: %T", v)
			}
		case "match":
			chk := &eval.EvaluateMatch{
				Trim:     true,
				Expected: v.(string),
			}
			evaluations[k] = chk
		case "contains":
			chk := &eval.EvaluateContains{
				Expected: v.(string),
			}
			evaluations[k] = chk
		default:
			return nil, ErrUnknownCheckType
		}
	}

	base.evaluations = &evaluations

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

func (t *ExecutionTest) Run(updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error) {

	// TODO: Should have an error channel to returns errors in.
	//   1. Failures during pre-execute should fail the test
	//   2. Failures during tests should fail the test
	//   3. Post-execute should always run even with a previous failure as it's part of the cleanup
	//   4. Failure in post-execute should stop tests as system will be in an unknown state at that point.

	results = make(map[string]*eval.EvaluateResult)

	// Run pre-execute commands
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
	testResult, testErr := t.node.Execute(t.execute)

	// Run post-execute commands
	updater.Update("cleanup")
	for _, cmd := range t.teardown {
		_, err = t.node.Execute(cmd)
		if err != nil {
			updater.Error()
			return nil, err
		}
	}

	if testErr != nil {
		updater.Error()
		return nil, err
	}

	passed := []bool{}
	for name, check := range *t.evaluations {
		result := check.Verify(testResult)
		if result.Passed == true {
			passed = append(passed, true)
		} else {
			passed = append(passed, false)
		}
		results[name] = result
	}

	updater.Complete(passed)
	return results, nil
}
