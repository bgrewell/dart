package internal

import (
	"fmt"
	"strings"
	"strconv"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

func NewTestController(
	suite string,
	platforms []ifaces.PlatformManager,
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
		Suite:        suite,
		Nodes:        nodes,
		Tests:        tests,
		Setup:        setup,
		Teardown:     teardown,
		Platforms:    platforms,
		formatter:    formatter,
		verbose:      verbose,
		stopOnFail:   stopOnFail,
		pauseOnFail:  pauseOnFail,
		setupOnly:    setupOnly,
		teardownOnly: teardownOnly,
	}
}

type TestController struct {
	Suite        string
	Nodes        map[string]ifaces.Node
	Setup        []ifaces.Step
	Tests        []ifaces.Test
	Teardown     []ifaces.Step
	Platforms    []ifaces.PlatformManager
	formatter    formatters.Formatter
	verbose      bool
	stopOnFail   bool
	pauseOnFail  bool
	setupOnly    bool
	teardownOnly bool
}

// handleSetupError handles errors during setup phases when pauseOnFail is enabled.
// Returns (retry, continue) - if retry is true, the step should be retried.
// If continue is true, skip the step and continue. If both are false, abort.
func (tc *TestController) handleSetupError(stepName string, err error) (retry bool, cont bool) {
	if !tc.pauseOnFail {
		return false, false
	}

	fmt.Printf("\nSetup step '%s' failed. Options:\n", stepName)
	fmt.Println("  [c]ontinue - Skip and continue with setup/tests")
	fmt.Println("  [r]etry    - Retry this step")
	fmt.Println("  [q]uit     - Cleanup and exit")
	fmt.Print("Choice [c/r/q]: ")

	var input string
	fmt.Scanln(&input)

	switch strings.ToLower(strings.TrimSpace(input)) {
	case "c", "continue":
		return false, true
	case "r", "retry":
		return true, false
	default:
		return false, false
	}
}

func (tc *TestController) Run() error {

	nodeSetupMsg := "running setup"
	nodeTeardownMsg := "running teardown"

	// Setup completed nodes
	var setupCompletedNodes []string

	// Track which platforms have been set up for cleanup on error
	var setupCompletedPlatforms []ifaces.PlatformManager

	// Create a defer function to clean up after a failure/error
	cleanupComplete := false
	cleanupMsg := "cleaning up after error"
	defer func() {
		// This only runs if the normal cleanup didn't run due to an error
		if !cleanupComplete {
			tc.formatter.PrintHeader(cleanupMsg)
			for _, name := range setupCompletedNodes {
				node := tc.Nodes[name]
				c := tc.formatter.StartTask(nodeTeardownMsg, name, "running")
				err := node.Teardown()
				if err != nil {
					c.Error()
					fmt.Sprintf("Error cleaning up node %s: %s", name, err)
				}
				c.Complete()
			}
			// Teardown platforms in reverse order
			for i := len(setupCompletedPlatforms) - 1; i >= 0; i-- {
				platform := setupCompletedPlatforms[i]
				t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "", "running")
				_ = platform.Teardown()
				t.Complete()
			}
		}
	}()

	// Get the max length of the setup/teardown and the tests for formatting
	longestSetup := len(nodeSetupMsg)
	if len(nodeTeardownMsg) > longestSetup {
		longestSetup = len(nodeTeardownMsg)
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

	// Calculate the longest node name for alignment
	longestNodeName := 0
	for name := range tc.Nodes {
		if len(name) > longestNodeName {
			longestNodeName = len(name)
		}
	}
	tc.formatter.SetNodeNameWidth(longestNodeName)

	// If teardown only is set, skip the setup and tests
	if tc.teardownOnly {
		cleanupMsg = "Running teardown only"
		for name := range tc.Nodes {
			setupCompletedNodes = append(setupCompletedNodes, name)
		}
		// Track all configured platforms for teardown
		for _, platform := range tc.Platforms {
			if platform.Configured() {
				setupCompletedPlatforms = append(setupCompletedPlatforms, platform)
			}
		}
		return nil // The defered function will handle the teardown
	}

	// Run the setup steps
	tc.formatter.PrintHeader("Running test setup")

	// Setup all configured platforms (e.g., Docker, LXD) before setting up nodes
	for _, platform := range tc.Platforms {
		if platform.Configured() {
			stepName := fmt.Sprintf("setting up %s environment", platform.Name())
		platformRetry:
			for {
				t := tc.formatter.StartTask(stepName, "", "running")
				err := platform.Setup()
				if err != nil {
					t.Error()
					tc.formatter.PrintError(err)
					retry, cont := tc.handleSetupError(stepName, err)
					if retry {
						continue platformRetry
					}
					if cont {
						break platformRetry
					}
					return err
				}
				setupCompletedPlatforms = append(setupCompletedPlatforms, platform)
				t.Complete()
				break
			}
		}
	}

	for name, node := range tc.Nodes {
	nodeRetry:
		for {
			c := tc.formatter.StartTask(nodeSetupMsg, name, "running")
			err := node.Setup()
			if err != nil {
				c.Error()
				tc.formatter.PrintError(err)
				retry, cont := tc.handleSetupError(fmt.Sprintf("node '%s' setup", name), err)
				if retry {
					continue nodeRetry
				}
				if cont {
					break nodeRetry
				}
				return err
			}
			setupCompletedNodes = append(setupCompletedNodes, name)
			c.Complete()
			break
		}
	}

	if len(tc.Setup) > 0 {
		for _, step := range tc.Setup {
		stepRetry:
			for {
				f := tc.formatter.StartTask(step.Title(), step.NodeName(), "running")
				err := step.Run(f)
				if err != nil {
					f.Error()
					tc.formatter.PrintError(err)
					retry, cont := tc.handleSetupError(step.Title(), err)
					if retry {
						continue stepRetry
					}
					if cont {
						break stepRetry
					}
					return err
				}
				break
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
		f := tc.formatter.StartTest(strconv.Itoa(id), test.Name(), test.NodeName())
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
			f := tc.formatter.StartTask(step.Title(), step.NodeName(), "running")
			err := step.Run(f)
			if err != nil {
				return err
			}
		}
	}

	for name, node := range tc.Nodes {
		c := tc.formatter.StartTask(nodeTeardownMsg, name, "running")
		err := node.Teardown()
		if err != nil {
			c.Error()
			return err
		}
		c.Complete()
	}

	// Teardown all configured platforms in reverse order
	for i := len(tc.Platforms) - 1; i >= 0; i-- {
		platform := tc.Platforms[i]
		if platform.Configured() {
			t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "", "running")
			err := platform.Teardown()
			if err != nil {
				t.Error()
				tc.formatter.PrintError(err)
				return err
			}
			t.Complete()
		}
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
