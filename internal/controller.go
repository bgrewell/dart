package internal

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/facts"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/report"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/steptypes"
	"github.com/bgrewell/dart/pkg/testtypes"
)

func NewTestController(
	suite string,
	platforms []ifaces.PlatformManager,
	nodes map[string]ifaces.Node,
	nodeConfigs []*config.NodeConfig,
	setupConfigs []*config.StepConfig,
	teardownConfigs []*config.StepConfig,
	testConfigs []*config.TestConfig,
	verbose bool,
	debug bool,
	stopOnFail bool,
	pauseOnFail bool,
	setupOnly bool,
	teardownOnly bool,
	until string,
	untilBehavior string,
	formatter formatters.Formatter) *TestController {

	// Set global debug mode for streaming output
	execution.SetDebugMode(debug)

	return &TestController{
		Suite:           suite,
		Nodes:           nodes,
		NodeConfigs:     nodeConfigs,
		SetupConfigs:    setupConfigs,
		TeardownConfigs: teardownConfigs,
		TestConfigs:     testConfigs,
		Platforms:       platforms,
		formatter:       formatter,
		verbose:         verbose,
		debug:           debug,
		stopOnFail:      stopOnFail,
		pauseOnFail:     pauseOnFail,
		setupOnly:       setupOnly,
		teardownOnly:    teardownOnly,
		until:           until,
		untilBehavior:   untilBehavior,
	}
}

type TestController struct {
	Suite             string
	Nodes             map[string]ifaces.Node
	NodeConfigs       []*config.NodeConfig
	SetupConfigs      []*config.StepConfig
	TeardownConfigs   []*config.StepConfig
	TestConfigs       []*config.TestConfig
	Setup             []ifaces.Step
	Tests             []ifaces.Test
	Teardown          []ifaces.Step
	Platforms         []ifaces.PlatformManager
	formatter         formatters.Formatter
	reports           []report.Spec
	reportIteration   int
	onlyTags          []string
	skipTags          []string
	filteredTests     []string
	filterExcludedAll bool
	verbose           bool
	debug             bool
	stopOnFail        bool
	pauseOnFail       bool
	setupOnly         bool
	teardownOnly      bool
	until             string
	untilBehavior     string
}

// SetReports configures machine-readable result outputs written after the
// suite completes.
func (tc *TestController) SetReports(specs []report.Spec) {
	tc.reports = specs
}

// orderedNodeNames returns node names in config-file order so setup and
// teardown are deterministic; nodes without a config entry (not expected)
// are appended in sorted order.
func (tc *TestController) orderedNodeNames() []string {
	names := make([]string, 0, len(tc.Nodes))
	seen := make(map[string]bool, len(tc.Nodes))
	for _, cfg := range tc.NodeConfigs {
		if _, ok := tc.Nodes[cfg.Name]; ok && !seen[cfg.Name] {
			names = append(names, cfg.Name)
			seen[cfg.Name] = true
		}
	}
	var rest []string
	for name := range tc.Nodes {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(names, rest...)
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

// validateUntilTarget checks that the --until target matches a setup step name,
// test name, or test index. Returns an error listing available names if no match is found.
func (tc *TestController) validateUntilTarget() error {
	if tc.until == "" {
		return nil
	}

	// Check setup step names
	for _, cfg := range tc.SetupConfigs {
		if cfg.Name == tc.until {
			return nil
		}
	}

	// Check test names and indices
	for idx, cfg := range tc.TestConfigs {
		if cfg.Name == tc.until || strconv.Itoa(idx+1) == tc.until {
			return nil
		}
	}

	// The target may exist in the suite but be excluded by tag filters —
	// say so instead of claiming it doesn't exist
	for _, name := range tc.filteredTests {
		if name == tc.until {
			return fmt.Errorf("--until target %q exists but is excluded by the --only/--skip tag filter", tc.until)
		}
	}

	// No match — build a helpful error message
	var names []string
	for _, cfg := range tc.SetupConfigs {
		names = append(names, fmt.Sprintf("  setup: %q", cfg.Name))
	}
	for idx, cfg := range tc.TestConfigs {
		names = append(names, fmt.Sprintf("  test %d: %q", idx+1, cfg.Name))
	}
	return fmt.Errorf("--until target %q not found. Available steps and tests:\n%s", tc.until, strings.Join(names, "\n"))
}

// applyUntilBehavior handles the --until stop point. Returns true if execution
// should stop (exit behavior), false if it should continue (pause behavior).
func (tc *TestController) applyUntilBehavior() bool {
	if tc.untilBehavior == "pause" {
		fmt.Printf("\nReached --until target %q. Press enter to continue execution...\n", tc.until)
		var input string
		fmt.Scanln(&input)
		return false
	}
	// Default: exit
	return true
}

// createStepsAndTests processes templates through the fact store, then creates
// the step and test objects from configs. Must be called after nodes are set up
// and facts are gathered.
func (tc *TestController) createStepsAndTests(store facts.FactStore) error {
	var err error

	// Process templates in configs
	if store != nil {
		tc.SetupConfigs, err = facts.ProcessStepConfigs(tc.SetupConfigs, store)
		if err != nil {
			return fmt.Errorf("processing setup templates: %w", err)
		}
		tc.TeardownConfigs, err = facts.ProcessStepConfigs(tc.TeardownConfigs, store)
		if err != nil {
			return fmt.Errorf("processing teardown templates: %w", err)
		}
		tc.TestConfigs, err = facts.ProcessTestConfigs(tc.TestConfigs, store)
		if err != nil {
			return fmt.Errorf("processing test templates: %w", err)
		}
	}

	// Create step and test objects
	tc.Setup, err = steptypes.CreateSteps(tc.SetupConfigs, tc.Nodes)
	if err != nil {
		return err
	}
	tc.Teardown, err = steptypes.CreateSteps(tc.TeardownConfigs, tc.Nodes)
	if err != nil {
		return err
	}
	tc.Tests, err = testtypes.CreateTests(tc.TestConfigs, tc.Nodes)
	if err != nil {
		return err
	}

	return nil
}

func (tc *TestController) Run() error {

	// Tag filters shape the test list before anything references it
	tc.applyTagFilters()
	if tc.filterExcludedAll {
		return fmt.Errorf("the --only/--skip tag filter excluded every test; nothing ran (check the tag names against the suite)")
	}

	// Validate --until target before doing any work
	if err := tc.validateUntilTarget(); err != nil {
		return err
	}

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
					fmt.Printf("Error cleaning up node %s: %s\n", name, err)
					continue
				}
				c.Complete()
			}
			// Teardown platforms in reverse order
			for i := len(setupCompletedPlatforms) - 1; i >= 0; i-- {
				platform := setupCompletedPlatforms[i]
				t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "", "running")
				if err := platform.Teardown(); err != nil {
					t.Error()
					fmt.Printf("Error cleaning up %s environment: %s\n", platform.Name(), err)
					continue
				}
				t.Complete()
			}
		}
	}()

	// Calculate the longest node name for alignment (available from configs)
	longestNodeName := 0
	for name := range tc.Nodes {
		if len(name) > longestNodeName {
			longestNodeName = len(name)
		}
	}
	tc.formatter.SetNodeNameWidth(longestNodeName)

	// Calculate task column width once, before displaying anything
	tc.formatter.SetTaskColumnWidth(tc.computeTaskColumnWidth(nodeSetupMsg, nodeTeardownMsg))

	// If teardown only is set, skip the setup and tests and run the full
	// teardown sequence: teardown steps, then nodes, then platforms. Steps
	// are best-effort — the remaining cleanup runs even if one fails.
	if tc.teardownOnly {
		// Create teardown steps without template processing (no facts available)
		var err error
		tc.Teardown, err = steptypes.CreateSteps(tc.TeardownConfigs, tc.Nodes)
		if err != nil {
			return err
		}

		// Compute test column width
		tc.setFormattingWidths()

		tc.formatter.PrintHeader("Running teardown only")
		for _, step := range tc.Teardown {
			f := tc.formatter.StartTask(step.Title(), step.NodeName(), "running")
			if err := step.Run(f); err != nil {
				f.Error()
				fmt.Printf("Error running teardown step %q: %s\n", step.Title(), err)
			}
		}

		for _, name := range tc.orderedNodeNames() {
			c := tc.formatter.StartTask(nodeTeardownMsg, name, "running")
			if err := tc.Nodes[name].Teardown(); err != nil {
				c.Error()
				fmt.Printf("Error cleaning up node %s: %s\n", name, err)
				continue
			}
			c.Complete()
		}

		for i := len(tc.Platforms) - 1; i >= 0; i-- {
			platform := tc.Platforms[i]
			if !platform.Configured() {
				continue
			}
			t := tc.formatter.StartTask(fmt.Sprintf("tearing down %s environment", platform.Name()), "", "running")
			if err := platform.Teardown(); err != nil {
				t.Error()
				fmt.Printf("Error cleaning up %s environment: %s\n", platform.Name(), err)
				continue
			}
			t.Complete()
		}

		cleanupComplete = true
		return nil
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

	for _, name := range tc.orderedNodeNames() {
		node := tc.Nodes[name]
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

	// Gather facts from nodes (after node setup, before step/test creation)
	var store facts.FactStore
	if facts.HasAnyFacts(tc.Nodes, tc.NodeConfigs) {
		// Built-in address facts are gathered for every capable node, but
		// only suites that ask for facts get the reporting phase — node
		// types gaining built-ins must not add output to existing suites
		showFacts := facts.HasFacts(tc.NodeConfigs)
		if showFacts {
			tc.formatter.PrintEmpty()
			tc.formatter.PrintHeader("Gathering node facts")
		}
		var err error
		store, err = facts.GatherFacts(tc.Nodes, tc.NodeConfigs)
		if err != nil {
			return err
		}
		if showFacts {
			for _, cfg := range tc.NodeConfigs {
				names := make([]string, 0, len(cfg.Facts))
				for factName := range cfg.Facts {
					names = append(names, factName)
				}
				sort.Strings(names)
				for _, factName := range names {
					f := tc.formatter.StartTask(factName, cfg.Name, "running")
					f.Complete()
				}
			}
		}
	}

	// Process templates and create steps/tests
	if err := tc.createStepsAndTests(store); err != nil {
		return err
	}

	// Compute test column width from the created test objects
	tc.setFormattingWidths()

	if len(tc.Setup) > 0 {
		untilReachedInSetup := false
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
			if tc.until != "" && step.Title() == tc.until {
				untilReachedInSetup = true
				break
			}
		}
		tc.formatter.PrintEmpty()
		if untilReachedInSetup {
			if tc.applyUntilBehavior() {
				cleanupComplete = true
				return nil
			}
		}
	}

	// If setup only is set, skip the tests and cleanup
	if tc.setupOnly {
		cleanupComplete = true
		return nil
	}

	// Run the tests. Results are collected per executed test in a slice —
	// test names are not unique (multi-node expansion reuses the name per
	// node), so a name-keyed map would collapse them in the summary.
	var testResults []map[string]*eval.EvaluateResult
	var records []report.TestRecord
	suiteStart := time.Now()
	skippedTests := 0

	// Guarantee a report on EVERY exit from here on — teardown failures,
	// --until exits, aborts. A CI run must never end reportless with
	// collected results; a missing artifact reads as "no results" rather
	// than what actually happened.
	reportWritten := false
	defer func() {
		if !reportWritten {
			tc.writeAbortReports(records, suiteStart)
		}
	}()
	tc.formatter.PrintHeader("Running tests")
	untilReachedInTests := false
	for idx, test := range tc.Tests {
		id := idx + 1
		f := tc.formatter.StartTest(strconv.Itoa(id), test.Name(), test.NodeName())

		// Skip conditions are evaluated before the test runs; a skipped
		// test is reported distinctly so it can never read as a pass. An
		// error in the condition itself fails the run.
		skip, skipReason, skipErr := test.ShouldSkip()
		if skipErr != nil {
			f.Error()
			tc.formatter.PrintFail(test.Name(), skipErr.Error())
			records = append(records, report.TestRecord{
				Name: test.Name(), Node: test.NodeName(),
				Status: report.StatusError, Failures: []string{skipErr.Error()},
			})
			return skipErr
		}
		if skip {
			f.Skip()
			skippedTests++
			records = append(records, report.TestRecord{
				Name: test.Name(), Node: test.NodeName(),
				Status: report.StatusSkip, Reason: skipReason,
			})
			if tc.verbose {
				tc.formatter.PrintSkip(test.Name(), skipReason)
			}
			if tc.until != "" && (test.Name() == tc.until || strconv.Itoa(id) == tc.until) {
				untilReachedInTests = true
				break
			}
			continue
		}

		testStart := time.Now()
		results, runErr := test.Run(f)
		record := report.TestRecord{
			Name: test.Name(), Node: test.NodeName(), Duration: time.Since(testStart),
		}

		// Results may be present alongside an error (teardown failure after
		// the test ran); record and report them before acting on the error
		if results != nil {
			testResults = append(testResults, results)

			names := make([]string, 0, len(results))
			for name := range results {
				names = append(names, name)
			}
			sort.Strings(names)

			// Report every check before acting on the outcome, so a test
			// with several failing checks shows all of them even under
			// stop-on-error, and pause-on-error prompts once per test
			testFailed := false
			for _, name := range names {
				result := results[name]
				if result.Err != nil {
					tc.formatter.PrintFail(name, fmt.Sprintf("evaluation error: %v", result.Err))
					record.Failures = append(record.Failures, fmt.Sprintf("%s: evaluation error: %v", name, result.Err))
				} else if result.Passed && tc.verbose {
					tc.formatter.PrintPass(name, result.Details)
				} else if !result.Passed {
					tc.formatter.PrintFail(name, result.Details)
					record.Failures = append(record.Failures, fmt.Sprintf("%s: %s", name, formatDetails(result.Details)))
				}
				if !result.Passed {
					testFailed = true
				}
			}
			switch {
			case len(results) == 0:
				record.Status = report.StatusRan
			case testFailed:
				record.Status = report.StatusFail
			default:
				record.Status = report.StatusPass
			}
			records = append(records, record)
			if testFailed {
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

		if runErr != nil {
			// TODO: This is an error not a fail, there should be a distinction since they are handled differently
			tc.formatter.PrintFail(test.Name(), runErr.Error())
			if results == nil {
				// The test never produced results; record the error itself
				record.Status = report.StatusError
				record.Failures = append(record.Failures, runErr.Error())
				records = append(records, record)
			}
			if tc.pauseOnFail {
				fmt.Println("Press enter to continue")
				var input string
				fmt.Scanln(&input)
			}
			return runErr
		}

		if tc.until != "" && (test.Name() == tc.until || strconv.Itoa(id) == tc.until) {
			untilReachedInTests = true
			break
		}
	}
	tc.formatter.PrintEmpty()
	if untilReachedInTests {
		if tc.applyUntilBehavior() {
			cleanupComplete = true
			return nil
		}
	}

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

	for _, name := range tc.orderedNodeNames() {
		c := tc.formatter.StartTask(nodeTeardownMsg, name, "running")
		err := tc.Nodes[name].Teardown()
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
	suiteElapsed := time.Since(suiteStart)
	tc.formatter.PrintResults(passed, failed, skippedTests, ran, suiteElapsed)
	cleanupComplete = true

	if err := tc.writeReports(records, suiteElapsed); err != nil {
		return err
	}
	reportWritten = true

	if failed > 0 {
		return fmt.Errorf("%d tests failed", failed)
	}
	return nil
}

// writeAbortReports best-effort writes reports when the run aborts early:
// a CI job that stops on error still needs a result file — a missing file
// reads as "no results" rather than "failed". Totals derive from the
// records collected so far; write errors are reported but do not mask the
// abort cause.
func (tc *TestController) writeAbortReports(records []report.TestRecord, suiteStart time.Time) {
	if len(tc.reports) == 0 {
		return
	}
	r := report.FromRecords(tc.Suite, records, time.Since(suiteStart))
	for _, spec := range tc.reports {
		if err := report.Write(tc.iterationSpec(spec), r); err != nil {
			fmt.Printf("Warning: writing %s report to %s: %s\n", spec.Format, spec.Path, err)
		}
	}
}

// writeReports renders configured machine-readable outputs. Totals derive
// from the records themselves so the file can never disagree with its own
// test list.
func (tc *TestController) writeReports(records []report.TestRecord, elapsed time.Duration) error {
	if len(tc.reports) == 0 {
		return nil
	}
	r := report.FromRecords(tc.Suite, records, elapsed)
	for _, spec := range tc.reports {
		if err := report.Write(tc.iterationSpec(spec), r); err != nil {
			return fmt.Errorf("writing %s report to %s: %w", spec.Format, spec.Path, err)
		}
	}
	return nil
}

// SetTagFilters restricts which tests run: with onlyTags set, a test must
// carry at least one of them; a test carrying any skipTags is excluded.
// Steps are never filtered — setup/teardown chains stay intact.
func (tc *TestController) SetTagFilters(onlyTags, skipTags []string) {
	tc.onlyTags = onlyTags
	tc.skipTags = skipTags
}

// applyTagFilters drops filtered tests from TestConfigs before creation,
// reporting how many were excluded.
func (tc *TestController) applyTagFilters() {
	if len(tc.onlyTags) == 0 && len(tc.skipTags) == 0 {
		return
	}
	hasAny := func(tags, wanted []string) bool {
		for _, tag := range tags {
			for _, want := range wanted {
				if tag == want {
					return true
				}
			}
		}
		return false
	}
	kept := make([]*config.TestConfig, 0, len(tc.TestConfigs))
	for _, cfg := range tc.TestConfigs {
		if len(tc.onlyTags) > 0 && !hasAny(cfg.Tags, tc.onlyTags) {
			continue
		}
		if len(tc.skipTags) > 0 && hasAny(cfg.Tags, tc.skipTags) {
			continue
		}
		kept = append(kept, cfg)
	}
	for _, cfg := range tc.TestConfigs {
		found := false
		for _, keptCfg := range kept {
			if keptCfg == cfg {
				found = true
				break
			}
		}
		if !found {
			tc.filteredTests = append(tc.filteredTests, cfg.Name)
		}
	}
	if excluded := len(tc.TestConfigs) - len(kept); excluded > 0 {
		tc.formatter.PrintHeader(fmt.Sprintf("Tag filter: running %d of %d tests (%d excluded)", len(kept), len(tc.TestConfigs), excluded))
	}
	// Filtering everything away is almost always a mistyped tag. Reporting
	// success for a run that executed nothing is the worst kind of green,
	// so it is recorded here and turned into an error by Run.
	tc.filterExcludedAll = len(tc.TestConfigs) > 0 && len(kept) == 0
	tc.TestConfigs = kept
}

// SetReportIteration marks which -i iteration is running (1-based) so each
// iteration writes its own report file instead of overwriting the last —
// a passing final iteration must not mask an earlier failure. Zero means
// single-run (paths unchanged).
func (tc *TestController) SetReportIteration(iteration int) {
	tc.reportIteration = iteration
}

// iterationSpec suffixes the report path with the iteration number
// (results.xml -> results-2.xml) when iterations are in play.
func (tc *TestController) iterationSpec(spec report.Spec) report.Spec {
	if tc.reportIteration <= 0 {
		return spec
	}
	ext := filepath.Ext(spec.Path)
	spec.Path = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(spec.Path, ext), tc.reportIteration, ext)
	return spec
}

// formatDetails renders evaluation details for report bodies.
func formatDetails(details interface{}) string {
	if details == nil {
		return "failed"
	}
	return fmt.Sprint(details)
}

// computeTaskColumnWidth calculates the maximum width needed for the task column
// by considering all items that use StartTask(): fact names, node setup/teardown
// messages, step titles, and platform messages.
func (tc *TestController) computeTaskColumnWidth(nodeSetupMsg, nodeTeardownMsg string) int {
	maxWidth := len(nodeSetupMsg)
	if len(nodeTeardownMsg) > maxWidth {
		maxWidth = len(nodeTeardownMsg)
	}

	// Include fact names
	for _, cfg := range tc.NodeConfigs {
		for factName := range cfg.Facts {
			if len(factName) > maxWidth {
				maxWidth = len(factName)
			}
		}
	}

	// Include step titles from configs
	for _, cfg := range tc.SetupConfigs {
		if len(cfg.Name) > maxWidth {
			maxWidth = len(cfg.Name)
		}
	}
	for _, cfg := range tc.TeardownConfigs {
		if len(cfg.Name) > maxWidth {
			maxWidth = len(cfg.Name)
		}
	}

	// Include platform messages
	for _, platform := range tc.Platforms {
		if platform.Configured() {
			setupMsg := fmt.Sprintf("setting up %s environment", platform.Name())
			teardownMsg := fmt.Sprintf("tearing down %s environment", platform.Name())
			if len(setupMsg) > maxWidth {
				maxWidth = len(setupMsg)
			}
			if len(teardownMsg) > maxWidth {
				maxWidth = len(teardownMsg)
			}
		}
	}

	return maxWidth
}

// setFormattingWidths sets the test column width based on the created test objects.
// Task column width should already be set by computeTaskColumnWidth().
func (tc *TestController) setFormattingWidths() {
	longestTest := 0
	for _, test := range tc.Tests {
		if len(test.Name()) > longestTest {
			longestTest = len(test.Name())
		}
	}
	tc.formatter.SetTestColumnWidth(longestTest)
}

func (tc *TestController) Close() error {
	var errs []error
	for name, node := range tc.Nodes {
		if err := node.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing node %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
