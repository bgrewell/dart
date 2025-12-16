package internal

import (
	"fmt"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"strconv"
)

func NewTestController(
	suite string,
	wrapper *docker.Wrapper,
	nodes map[string]ifaces.Node,
	tests []ifaces.Test,
	setup []ifaces.Step,
	teardown []ifaces.Step,
	verbose bool,
	stopOnFail bool,
	pauseOnFail bool,
	setupOnly bool,
	teardownOnly bool,
	formatter formatters.Formatter) *TestController {
	return &TestController{
		Suite:         suite,
		Nodes:         nodes,
		Tests:         tests,
		Setup:         setup,
		Teardown:      teardown,
		DockerWrapper: wrapper,
		formatter:     formatter,
		verbose:       verbose,
		stopOnFail:    stopOnFail,
		pauseOnFail:   pauseOnFail,
		setupOnly:     setupOnly,
		teardownOnly:  teardownOnly,
	}
}

type TestController struct {
	Suite         string
	Nodes         map[string]ifaces.Node
	Setup         []ifaces.Step
	Tests         []ifaces.Test
	Teardown      []ifaces.Step
	DockerWrapper *docker.Wrapper
	formatter     formatters.Formatter
	verbose       bool
	stopOnFail    bool
	pauseOnFail   bool
	setupOnly     bool
	teardownOnly  bool
}

func (tc *TestController) Run() error {

	nodeSetupMsg := "running setup on %s"
	nodeTeardownMsg := "running teardown on %s"

	// Setup completed nodes
	var setupCompletedNodes []string

	// Create a defer function to clean up after a failure/error
	cleanupComplete := false
	cleanupMsg := "cleaning up after error"
	defer func() {
		// This only runs if the normal cleanup didn't run due to an error
		if !cleanupComplete {
			tc.formatter.PrintHeader(cleanupMsg)
			for _, name := range setupCompletedNodes {
				node := tc.Nodes[name]
				c := tc.formatter.StartTask(fmt.Sprintf(nodeTeardownMsg, name), "running")
				err := node.Teardown()
				if err != nil {
					c.Error()
					fmt.Sprintf("Error cleaning up node %s: %s", name, err)
				}
				c.Complete()
			}
			if tc.DockerWrapper.Configured() {
				t := tc.formatter.StartTask("tearing down docker environment", "running")
				_ = tc.DockerWrapper.Teardown()
				t.Complete()
			}
		}
	}()

	// Get the max length of the setup/teardown and the tests for formatting
	longestSetup := 0
	for name, _ := range tc.Nodes {
		if len(fmt.Sprintf(nodeSetupMsg, name)) > longestSetup {
			longestSetup = len(fmt.Sprintf(nodeSetupMsg, name))
		}
		if len(fmt.Sprintf(nodeTeardownMsg, name)) > longestSetup {
			longestSetup = len(fmt.Sprintf(nodeTeardownMsg, name))
		}
	}
	for _, step := range append(tc.Setup, tc.Teardown...) {
		if len(step.Title()) > longestSetup {
			longestSetup = len(step.Title())
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

	// If teardown only is set, skip the setup and tests
	if tc.teardownOnly {
		cleanupMsg = "Running teardown only"
		for name, _ := range tc.Nodes {
			setupCompletedNodes = append(setupCompletedNodes, name)
		}
		return nil // The defered function will handle the teardown
	}

	// Run the setup steps
	tc.formatter.PrintHeader("Running test setup")

	// Check if the docker wrapper is configured and if it is then run the docker setup steps
	if tc.DockerWrapper.Configured() {
		// Run the docker set up steps
		t := tc.formatter.StartTask("setting up docker environment", "running")
		err := tc.DockerWrapper.Setup()
		if err != nil {
			t.Error()
			tc.formatter.PrintError(err)
			return err
		}
		t.Complete()
	}

	for name, node := range tc.Nodes {
		c := tc.formatter.StartTask(fmt.Sprintf(nodeSetupMsg, name), "running")
		err := node.Setup()
		if err != nil {
			c.Error()
			tc.formatter.PrintError(err)
			return err
		}
		setupCompletedNodes = append(setupCompletedNodes, name)
		c.Complete()
	}

	if len(tc.Setup) > 0 {
		for _, step := range tc.Setup {
			f := tc.formatter.StartTask(step.Title(), "running")
			err := step.Run(f)
			if err != nil {
				f.Error()
				tc.formatter.PrintError(err)
				return err
			}
		}
		tc.formatter.PrintEmpty()
	}

	// If setup only is set, skip the tests and cleanup
	if tc.setupOnly {
		cleanupComplete = true
		return nil
	}

	// Run the tests
	testResults := make(map[string]map[string]*eval.EvaluateResult)
	tc.formatter.PrintHeader("Running tests")
	for idx, test := range tc.Tests {
		id := idx + 1
		f := tc.formatter.StartTest(strconv.Itoa(id), test.Name())
		results, err := test.Run(f)
		if err != nil {
			// TODO: This is an error not a fail, there should be a distinction since they are handled differently
			tc.formatter.PrintFail(test.Name(), err.Error())
			if tc.pauseOnFail {
				fmt.Println("Press enter to continue")
				var input string
				fmt.Scanln(&input)
			}
			return err
		}
		testResults[test.Name()] = results

		for name, result := range results {
			if result.Passed && tc.verbose {
				tc.formatter.PrintPass(name, result.Details)
			} else if !result.Passed {
				tc.formatter.PrintFail(name, result.Details)
				if tc.stopOnFail {
					return fmt.Errorf("test %s failed", test.Name())
				}
				if tc.pauseOnFail {
					fmt.Println("Press enter to continue")
					var input string
					fmt.Scanln(&input)
				}
			}
		}
	}
	tc.formatter.PrintEmpty()

	// Run the teardown steps
	tc.formatter.PrintHeader("Running test teardown")
	if len(tc.Teardown) > 0 {
		for _, step := range tc.Teardown {
			f := tc.formatter.StartTask(step.Title(), "running")
			err := step.Run(f)
			if err != nil {
				return err
			}
		}
	}

	for name, node := range tc.Nodes {
		c := tc.formatter.StartTask(fmt.Sprintf(nodeTeardownMsg, name), "running")
		err := node.Teardown()
		if err != nil {
			c.Error()
			return err
		}
		c.Complete()
	}

	if tc.DockerWrapper.Configured() {
		// Run the docker teardown steps
		t := tc.formatter.StartTask("tearing down docker environment", "running")
		err := tc.DockerWrapper.Teardown()
		if err != nil {
			t.Error()
			tc.formatter.PrintError(err)
			return err
		}
		t.Complete()
	}
	tc.formatter.PrintEmpty()

	// Count the passes and fails and print the test results
	passed, failed, ran := 0, 0, 0
	for _, results := range testResults {

		if len(results) == 0 {
			ran++
			continue
		}

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
	tc.formatter.PrintResults(passed, failed, ran)
	cleanupComplete = true

	if failed > 0 {
		return fmt.Errorf("%d tests failed", failed)
	}
	return nil
}

func (tc *TestController) Close() error {
	for _, node := range tc.Nodes {
		node.Close()
	}
	return nil
}
