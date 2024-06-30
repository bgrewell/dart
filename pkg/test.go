package pkg

import (
	"github.com/bgrewell/dart/internal"
	"github.com/bgrewell/dart/internal/check"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"sort"
)

type Test interface {
	Name() string
	Run(updater formatters.TestCompleter) (results map[string]*check.CheckResult, err error)
}

type ExecutionTest struct {
	name        string
	node        Node
	preExecute  []string
	execute     string
	postExecute []string
	checks      map[string]check.Check
}

func (t *ExecutionTest) Name() string {
	return t.name
}

func (t *ExecutionTest) Run(updater formatters.TestCompleter) (results map[string]*check.CheckResult, err error) {

	// TODO: Should have an error channel to returns errors in.
	//   1. Failures during pre-execute should fail the test
	//   2. Failures during tests should fail the test
	//   3. Post-execute should always run even with a previous failure as it's part of the cleanup
	//   4. Failure in post-execute should stop tests as system will be in an unknown state at that point.

	results = make(map[string]*check.CheckResult)

	// Run pre-execute commands
	updater.Update("preparing")
	for _, cmd := range t.preExecute {
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
	for _, cmd := range t.postExecute {
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
	for name, check := range t.checks {
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

// CreateTests creates a slice of Test objects from a slice of TestConfig objects
func CreateTests(configs []*config.TestConfig, nodes map[string]Node) (tests []Test, err error) {
	tests = make([]Test, 0)

	// Sort tests by order
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Order < configs[j].Order
	})

	for _, cfg := range configs {
		node, ok := nodes[cfg.Node]
		if !ok {
			return nil, internal.ErrNodeNotFound
		}

		checks := make(map[string]check.Check)
		for k, v := range cfg.Check {
			switch k {
			case "exit_code":
				chk := &check.ExitCodeCheck{
					Expected: v.(int),
				}
				checks[k] = chk
			case "match":
				chk := &check.MatchCheck{
					Trim:     true,
					Expected: v.(string),
				}
				checks[k] = chk
			case "contains":
				chk := &check.ContainsCheck{
					Expected: v.(string),
				}
				checks[k] = chk
			default:
				return nil, internal.ErrUnknownCheckType
			}
		}

		tests = append(tests, &ExecutionTest{
			name:        cfg.Name,
			node:        node,
			preExecute:  cfg.PreExecute,
			execute:     cfg.Execute,
			postExecute: cfg.PostExecute,
			checks:      checks,
		})
	}
	return tests, nil
}
