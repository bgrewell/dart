package pkg

import (
	"github.com/bgrewell/dart/internal/check"
	"github.com/bgrewell/dart/internal/formatters"
	"strconv"
)

func NewTestController(suite string, nodes map[string]Node, tests []Test, setup []Step, teardown []Step, formatter formatters.Formatter) *TestController {
	return &TestController{
		Suite:     suite,
		Nodes:     nodes,
		Tests:     tests,
		Setup:     setup,
		Teardown:  teardown,
		formatter: formatter,
		verbose:   false,
	}
}

type TestController struct {
	Suite     string
	Nodes     map[string]Node
	Setup     []Step
	Tests     []Test
	Teardown  []Step
	formatter formatters.Formatter
	verbose   bool
}

func (tc *TestController) Run() error {

	// Get the max length of the setup/teardown and the tests
	longestSetup := 0
	for _, step := range append(tc.Setup, tc.Teardown...) {
		if step.TitleLen() > longestSetup {
			longestSetup = step.TitleLen()
		}
	}
	tc.formatter.SetTaskColumnWidth(longestSetup)

	longestTest := 0
	for _, test := range tc.Tests {
		if len(test.Name()) > longestTest {
			longestTest = len(test.Name())
		}
	}
	tc.formatter.SetTestColumnWidth(longestTest)

	// Run the setup steps
	if len(tc.Setup) > 0 {
		tc.formatter.PrintHeader("Running test setup")
		for _, step := range tc.Setup {
			f := tc.formatter.StartTask(step.Title(), "running")
			err := step.Run(f)
			if err != nil {
				return err
			}
		}
		tc.formatter.PrintEmpty()
	}

	// Run the tests
	testResults := make(map[string]map[string]*check.CheckResult)
	tc.formatter.PrintHeader("Running tests")
	for idx, test := range tc.Tests {
		id := idx + 1
		f := tc.formatter.StartTest(strconv.Itoa(id), test.Name())
		results, err := test.Run(f)
		if err != nil {
			return err
		}
		testResults[test.Name()] = results

		for name, result := range results {
			if result.Passed && tc.verbose {
				tc.formatter.PrintPass(name, result.Details)
			} else if !result.Passed {
				tc.formatter.PrintFail(name, result.Details)
			}
		}
	}
	tc.formatter.PrintEmpty()

	// Run the teardown steps
	if len(tc.Teardown) > 0 {
		tc.formatter.PrintHeader("Running test teardown")
		for _, step := range tc.Teardown {
			f := tc.formatter.StartTask(step.Title(), "running")
			err := step.Run(f)
			if err != nil {
				return err
			}
		}
		tc.formatter.PrintEmpty()
	}

	// Count the passes and fails and print the test results
	passed, failed := 0, 0
	for _, results := range testResults {
		// Count the tests, not the checks so any failed check is a failed test
		testPassed := true
		for _, result := range results {
			if !result.Passed {
				testPassed = false
				break
			}
		}
		if testPassed {
			passed++
		} else {
			failed++
		}
	}
	tc.formatter.PrintResults(passed, failed)

	return nil
}

func (tc *TestController) Close() error {
	for _, node := range tc.Nodes {
		node.Close()
	}
	return nil
}
