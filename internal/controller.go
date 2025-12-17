package internal

import (
	"fmt"
	"sync"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"strconv"
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

// executeStepsInParallel groups steps by node and executes them in parallel.
// Steps for the same node are executed sequentially to preserve order.
// Steps for different nodes are executed in parallel.
func (tc *TestController) executeStepsInParallel(steps []ifaces.Step) error {
	// Group steps by node
	stepsByNode := make(map[string][]ifaces.Step)
	for _, step := range steps {
		nodeName := step.NodeName()
		stepsByNode[nodeName] = append(stepsByNode[nodeName], step)
	}

	// Execute steps for each node in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(stepsByNode))

	for _, nodeSteps := range stepsByNode {
		wg.Add(1)
		go func(steps []ifaces.Step) {
			defer wg.Done()
			// Execute steps for this node sequentially
			for _, step := range steps {
				f := tc.formatter.StartTask(step.Title(), "running")
				err := step.Run(f)
				if err != nil {
					f.Error()
					tc.formatter.PrintError(err)
					errChan <- err
					return
				}
			}
		}(nodeSteps)
	}

	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

// executeNodesInParallel executes setup or teardown for all nodes in parallel.
func (tc *TestController) executeNodesInParallel(setupCompletedNodes *[]string, operation string) error {
	nodeSetupMsg := "running setup on %s"
	nodeTeardownMsg := "running teardown on %s"
	
	var wg sync.WaitGroup
	errChan := make(chan error, len(tc.Nodes))
	var mu sync.Mutex

	for name, node := range tc.Nodes {
		wg.Add(1)
		go func(nodeName string, n ifaces.Node) {
			defer wg.Done()
			
			if operation == "setup" {
				c := tc.formatter.StartTask(fmt.Sprintf(nodeSetupMsg, nodeName), "running")
				err := n.Setup()
				if err != nil {
					c.Error()
					tc.formatter.PrintError(err)
					errChan <- err
					return
				}
				mu.Lock()
				*setupCompletedNodes = append(*setupCompletedNodes, nodeName)
				mu.Unlock()
				c.Complete()
			} else if operation == "teardown" {
				c := tc.formatter.StartTask(fmt.Sprintf(nodeTeardownMsg, nodeName), "running")
				err := n.Teardown()
				if err != nil {
					c.Error()
					errChan <- err
					return
				}
				c.Complete()
			}
		}(name, node)
	}

	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

func (tc *TestController) Run() error {

	nodeSetupMsg := "running setup on %s"
	nodeTeardownMsg := "running teardown on %s"

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
				c := tc.formatter.StartTask(fmt.Sprintf(nodeTeardownMsg, name), "running")
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
				t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "running")
				_ = platform.Teardown()
				t.Complete()
			}
		}
	}()

	// Get the max length of the setup/teardown and the tests for formatting
	longestSetup := 0
	for name := range tc.Nodes {
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
			t := tc.formatter.StartTask(fmt.Sprintf("setting up %s environment", platform.Name()), "running")
			err := platform.Setup()
			if err != nil {
				t.Error()
				tc.formatter.PrintError(err)
				return err
			}
			setupCompletedPlatforms = append(setupCompletedPlatforms, platform)
			t.Complete()
		}
	}

	// Execute node setup in parallel
	err := tc.executeNodesInParallel(&setupCompletedNodes, "setup")
	if err != nil {
		return err
	}

	// Execute setup steps in parallel (grouped by node)
	if len(tc.Setup) > 0 {
		err := tc.executeStepsInParallel(tc.Setup)
		if err != nil {
			return err
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
		err := tc.executeStepsInParallel(tc.Teardown)
		if err != nil {
			return err
		}
	}

	// Execute node teardown in parallel
	var dummyNodes []string
	err = tc.executeNodesInParallel(&dummyNodes, "teardown")
	if err != nil {
		return err
	}

	// Teardown all configured platforms in reverse order
	for i := len(tc.Platforms) - 1; i >= 0; i-- {
		platform := tc.Platforms[i]
		if platform.Configured() {
			t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "running")
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
