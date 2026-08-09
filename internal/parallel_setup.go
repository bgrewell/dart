package internal

import (
	"fmt"
	"sync"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// setupGroup is one node's setup steps, in the order the suite declares them.
type setupGroup struct {
	node  string
	steps []ifaces.Step
}

// groupSetupByNode splits steps into per-node chains, preserving both the
// order of steps within a node and the order in which nodes first appear.
//
// Invariant: steps for one node stay sequential. Only different nodes run
// concurrently, because a node's own steps routinely depend on each other
// (install, then configure, then start).
func groupSetupByNode(steps []ifaces.Step) []setupGroup {
	var groups []setupGroup
	index := map[string]int{}

	for _, step := range steps {
		node := step.NodeName()
		at, seen := index[node]
		if !seen {
			index[node] = len(groups)
			groups = append(groups, setupGroup{node: node, steps: []ifaces.Step{step}})
			continue
		}
		groups[at].steps = append(groups[at].steps, step)
	}
	return groups
}

// parallelSetupSafe reports whether the run's flags allow concurrent setup,
// and why not when they do not.
//
// Rationale: three features assume one step runs at a time. --pause-on-error
// reads stdin, which concurrent prompts cannot share; --until names a step to
// stop at, which has no meaning once siblings are already in flight; and
// stopping at the first failure is only well defined when there is an order
// to stop in.
func parallelSetupSafe(pauseOnFail bool, until string) (bool, string) {
	if pauseOnFail {
		return false, "--pause-on-error reads stdin and cannot prompt for concurrent steps"
	}
	if until != "" {
		return false, "--until stops at a named step, which requires setup to run in order"
	}
	return true, ""
}

// runSetupGroupsParallel runs each node's chain concurrently and returns the
// first error in node-declaration order, so the reported failure does not
// depend on which goroutine happened to finish first.
//
// Every group runs to completion even after another fails: a half-configured
// node is harder to clean up than a fully configured one, and teardown runs
// either way.
func runSetupGroupsParallel(groups []setupGroup, line func(task, node string) string,
	newCompleter func(message string) formatters.TaskCompleter) []error {

	errs := make([]error, len(groups))
	var wg sync.WaitGroup

	for i, group := range groups {
		wg.Add(1)
		go func(i int, group setupGroup) {
			defer wg.Done()
			for _, step := range group.steps {
				completer := newCompleter(line(step.Title(), step.NodeName()))
				if err := step.Run(completer); err != nil {
					completer.Error()
					// The rest of this node's chain is skipped: later steps
					// assume the earlier ones succeeded.
					//
					// The node is named in the error because failures are
					// reported together after every group finishes, detached
					// from the task lines that show which node each belongs
					// to. The sequential path prints its error immediately
					// under that line and so does not need the prefix.
					errs[i] = fmt.Errorf("node %s: %w", group.node, err)
					return
				}
			}
		}(i, group)
	}
	wg.Wait()

	ordered := make([]error, 0, len(groups))
	for _, err := range errs {
		if err != nil {
			ordered = append(ordered, err)
		}
	}
	return ordered
}

// runParallelSetup runs the suite's setup steps grouped by node. It is only
// reached once parallelSetupSafe has approved the run's flags.
func (tc *TestController) runParallelSetup() []error {
	groups := groupSetupByNode(tc.Setup)
	if len(groups) <= 1 {
		// Nothing to overlap; the sequential path renders better
		return tc.runSetupGroupsSequentially(groups)
	}

	standard, ok := tc.formatter.(*formatters.StandardFormatter)
	if !ok {
		return tc.runSetupGroupsSequentially(groups)
	}

	var mu sync.Mutex
	return runSetupGroupsParallel(groups,
		standard.QuietTaskLine,
		func(message string) formatters.TaskCompleter {
			return formatters.NewQuietTaskCompleter(standard.Out(), &mu, message)
		})
}

// runSetupGroupsSequentially is the fallback when there is nothing to gain
// from concurrency, or when the formatter cannot render it.
func (tc *TestController) runSetupGroupsSequentially(groups []setupGroup) []error {
	var errs []error
	for _, group := range groups {
		for _, step := range group.steps {
			c := tc.formatter.StartTask(step.Title(), step.NodeName(), "running")
			if err := step.Run(c); err != nil {
				c.Error()
				errs = append(errs, fmt.Errorf("node %s: %w", group.node, err))
				break
			}
		}
	}
	return errs
}
