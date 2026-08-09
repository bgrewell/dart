package internal

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStep records when it ran and can fail or block on demand.
type fakeStep struct {
	title   string
	node    string
	err     error
	delay   time.Duration
	started *int32
	maxSeen *int32
	ran     *[]string
	ranMu   *sync.Mutex
}

func (f *fakeStep) Title() string    { return f.title }
func (f *fakeStep) NodeName() string { return f.node }

func (f *fakeStep) Run(updater formatters.TaskCompleter) error {
	if f.started != nil {
		inFlight := atomic.AddInt32(f.started, 1)
		for {
			peak := atomic.LoadInt32(f.maxSeen)
			if inFlight <= peak || atomic.CompareAndSwapInt32(f.maxSeen, peak, inFlight) {
				break
			}
		}
		defer atomic.AddInt32(f.started, -1)
	}
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.ran != nil {
		f.ranMu.Lock()
		*f.ran = append(*f.ran, f.node+"/"+f.title)
		f.ranMu.Unlock()
	}
	if f.err != nil {
		updater.Error()
		return f.err
	}
	updater.Complete()
	return nil
}

func runGroups(t *testing.T, steps []ifaces.Step) ([]error, string) {
	t.Helper()
	var out bytes.Buffer
	var mu sync.Mutex
	groups := groupSetupByNode(steps)
	errs := runSetupGroupsParallel(groups, nil,
		func(task, node string) string { return fmt.Sprintf("  [ %s ] %s ... ", node, task) },
		func(message string) formatters.TaskCompleter {
			return formatters.NewQuietTaskCompleter(&out, &mu, message)
		})
	return errs, out.String()
}

// Steps for one node must stay in order: a node's own steps routinely depend
// on each other.
func TestGroupSetupPreservesPerNodeOrder(t *testing.T) {
	steps := []ifaces.Step{
		&fakeStep{title: "install", node: "a"},
		&fakeStep{title: "install", node: "b"},
		&fakeStep{title: "configure", node: "a"},
		&fakeStep{title: "start", node: "a"},
		&fakeStep{title: "configure", node: "b"},
	}
	groups := groupSetupByNode(steps)

	require.Len(t, groups, 2)
	assert.Equal(t, "a", groups[0].node, "nodes keep first-appearance order")
	assert.Equal(t, "b", groups[1].node)

	titles := func(g setupGroup) []string {
		var out []string
		for _, s := range g.steps {
			out = append(out, s.Title())
		}
		return out
	}
	assert.Equal(t, []string{"install", "configure", "start"}, titles(groups[0]))
	assert.Equal(t, []string{"install", "configure"}, titles(groups[1]))
}

// Different nodes must actually overlap, or the feature does nothing.
func TestDifferentNodesRunConcurrently(t *testing.T) {
	var inFlight, peak int32
	steps := []ifaces.Step{
		&fakeStep{title: "slow", node: "a", delay: 60 * time.Millisecond, started: &inFlight, maxSeen: &peak},
		&fakeStep{title: "slow", node: "b", delay: 60 * time.Millisecond, started: &inFlight, maxSeen: &peak},
		&fakeStep{title: "slow", node: "c", delay: 60 * time.Millisecond, started: &inFlight, maxSeen: &peak},
	}

	start := time.Now()
	errs, _ := runGroups(t, steps)
	elapsed := time.Since(start)

	assert.Empty(t, errs)
	assert.GreaterOrEqual(t, int(atomic.LoadInt32(&peak)), 2, "at least two nodes must overlap")
	assert.Less(t, elapsed, 150*time.Millisecond, "three 60ms steps run sequentially would take ~180ms")
}

// A node's own steps must never overlap each other.
func TestSameNodeStepsDoNotOverlap(t *testing.T) {
	var inFlight, peak int32
	steps := []ifaces.Step{
		&fakeStep{title: "one", node: "a", delay: 30 * time.Millisecond, started: &inFlight, maxSeen: &peak},
		&fakeStep{title: "two", node: "a", delay: 30 * time.Millisecond, started: &inFlight, maxSeen: &peak},
		&fakeStep{title: "three", node: "a", delay: 30 * time.Millisecond, started: &inFlight, maxSeen: &peak},
	}

	errs, _ := runGroups(t, steps)
	assert.Empty(t, errs)
	assert.Equal(t, int32(1), atomic.LoadInt32(&peak), "one node's steps must be sequential")
}

// A failure stops that node's remaining chain — later steps assume the
// earlier ones succeeded — but must not stop the other nodes.
func TestFailureStopsOnlyItsOwnNode(t *testing.T) {
	var ran []string
	var ranMu sync.Mutex
	steps := []ifaces.Step{
		&fakeStep{title: "boom", node: "a", err: errors.New("install failed"), ran: &ran, ranMu: &ranMu},
		&fakeStep{title: "after", node: "a", ran: &ran, ranMu: &ranMu},
		&fakeStep{title: "fine", node: "b", ran: &ran, ranMu: &ranMu},
	}

	errs, _ := runGroups(t, steps)

	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "node a")
	assert.Contains(t, errs[0].Error(), "install failed")

	ranMu.Lock()
	defer ranMu.Unlock()
	assert.Contains(t, ran, "a/boom")
	assert.Contains(t, ran, "b/fine", "an unrelated node keeps running")
	assert.NotContains(t, ran, "a/after", "the rest of the failing node's chain is skipped")
}

// Reported errors follow node-declaration order, so the failure a user sees
// does not depend on which goroutine finished first.
func TestErrorsReportedInNodeOrder(t *testing.T) {
	steps := []ifaces.Step{
		// b is declared first but finishes last
		&fakeStep{title: "boom", node: "b", err: errors.New("b failed"), delay: 40 * time.Millisecond},
		&fakeStep{title: "boom", node: "a", err: errors.New("a failed")},
	}

	errs, _ := runGroups(t, steps)
	require.Len(t, errs, 2)
	assert.Contains(t, errs[0].Error(), "node b", "first declared node reports first")
	assert.Contains(t, errs[1].Error(), "node a")
}

// Lines must never interleave mid-write, and each task prints exactly once.
func TestOutputLinesAreWholeAndPrintedOnce(t *testing.T) {
	var steps []ifaces.Step
	for i := 0; i < 12; i++ {
		steps = append(steps, &fakeStep{title: fmt.Sprintf("task%02d", i), node: fmt.Sprintf("node%02d", i)})
	}

	_, out := runGroups(t, steps)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 12, "one line per task")
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "  [ node"), "line was cut: %q", line)
		assert.True(t, strings.HasSuffix(line, "done"), "line was cut: %q", line)
	}
}

// The flags that assume one step at a time must force the sequential path.
func TestParallelSetupSafety(t *testing.T) {
	ok, _ := parallelSetupSafe(false, "")
	assert.True(t, ok)

	ok, reason := parallelSetupSafe(true, "")
	assert.False(t, ok)
	assert.Contains(t, reason, "stdin")

	ok, reason = parallelSetupSafe(false, "install packages")
	assert.False(t, ok)
	assert.Contains(t, reason, "--until")
}
