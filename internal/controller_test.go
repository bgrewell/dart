package internal

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// MockStep is a mock implementation of the Step interface for testing
type MockStep struct {
	title    string
	nodeName string
	delay    time.Duration
	runErr   error
	runOrder *[]string
	mu       *sync.Mutex
}

func (m *MockStep) Run(updater formatters.TaskCompleter) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.runOrder != nil && m.mu != nil {
		m.mu.Lock()
		*m.runOrder = append(*m.runOrder, m.title)
		m.mu.Unlock()
	}
	return m.runErr
}

func (m *MockStep) Title() string {
	return m.title
}

func (m *MockStep) NodeName() string {
	return m.nodeName
}

func TestExecuteStepsInParallel(t *testing.T) {
	tests := []struct {
		name       string
		steps      []ifaces.Step
		wantErr    bool
		checkOrder func(order []string) bool
	}{
		{
			name: "steps on different nodes execute in parallel",
			steps: []ifaces.Step{
				&MockStep{title: "step1", nodeName: "node1", delay: 10 * time.Millisecond},
				&MockStep{title: "step2", nodeName: "node2", delay: 10 * time.Millisecond},
				&MockStep{title: "step3", nodeName: "node3", delay: 10 * time.Millisecond},
			},
			wantErr: false,
		},
		{
			name: "steps on same node execute sequentially",
			steps: func() []ifaces.Step {
				var runOrder []string
				var mu sync.Mutex
				return []ifaces.Step{
					&MockStep{title: "step1", nodeName: "node1", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
					&MockStep{title: "step2", nodeName: "node1", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
					&MockStep{title: "step3", nodeName: "node1", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
				}
			}(),
			wantErr: false,
			checkOrder: func(order []string) bool {
				// Steps on the same node should execute in order
				return len(order) == 3 &&
					order[0] == "step1" &&
					order[1] == "step2" &&
					order[2] == "step3"
			},
		},
		{
			name: "error in one step stops execution",
			steps: []ifaces.Step{
				&MockStep{title: "step1", nodeName: "node1", delay: 5 * time.Millisecond},
				&MockStep{title: "step2", nodeName: "node2", delay: 5 * time.Millisecond, runErr: errors.New("test error")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := formatters.NewStandardFormatter()
			formatter.SetTaskColumnWidth(50) // Set a reasonable width for testing
			
			tc := &TestController{
				formatter: formatter,
			}

			start := time.Now()
			err := tc.executeStepsInParallel(tt.steps)
			elapsed := time.Since(start)

			if (err != nil) != tt.wantErr {
				t.Errorf("executeStepsInParallel() error = %v, wantErr %v", err, tt.wantErr)
			}

			// For the parallel execution test, verify it didn't take too long
			if tt.name == "steps on different nodes execute in parallel" && !tt.wantErr {
				// If steps ran sequentially, it would take 30ms. In parallel, should be ~10ms
				if elapsed > 20*time.Millisecond {
					t.Errorf("Steps appear to have run sequentially (took %v), expected parallel execution", elapsed)
				}
			}

			// Check order if required
			if tt.checkOrder != nil {
				var runOrder []string
				for _, step := range tt.steps {
					if mockStep, ok := step.(*MockStep); ok && mockStep.runOrder != nil {
						runOrder = *mockStep.runOrder
						break
					}
				}
				if !tt.checkOrder(runOrder) {
					t.Errorf("Step execution order was incorrect: %v", runOrder)
				}
			}
		})
	}
}

func TestExecuteStepsInParallelGrouping(t *testing.T) {
	// Test that steps are properly grouped by node
	var runOrder []string
	var mu sync.Mutex

	steps := []ifaces.Step{
		&MockStep{title: "node1-step1", nodeName: "node1", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
		&MockStep{title: "node2-step1", nodeName: "node2", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
		&MockStep{title: "node1-step2", nodeName: "node1", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
		&MockStep{title: "node2-step2", nodeName: "node2", delay: 5 * time.Millisecond, runOrder: &runOrder, mu: &mu},
	}

	tc := &TestController{
		formatter: func() formatters.Formatter {
			f := formatters.NewStandardFormatter()
			f.SetTaskColumnWidth(50)
			return f
		}(),
	}

	err := tc.executeStepsInParallel(steps)
	if err != nil {
		t.Fatalf("executeStepsInParallel() unexpected error = %v", err)
	}

	// Verify that steps for the same node executed in order
	node1Index1 := -1
	node1Index2 := -1
	node2Index1 := -1
	node2Index2 := -1

	for i, step := range runOrder {
		switch step {
		case "node1-step1":
			node1Index1 = i
		case "node1-step2":
			node1Index2 = i
		case "node2-step1":
			node2Index1 = i
		case "node2-step2":
			node2Index2 = i
		}
	}

	// For node1, step1 should come before step2
	if node1Index1 >= node1Index2 {
		t.Errorf("node1 steps executed out of order: step1 at %d, step2 at %d", node1Index1, node1Index2)
	}

	// For node2, step1 should come before step2
	if node2Index1 >= node2Index2 {
		t.Errorf("node2 steps executed out of order: step1 at %d, step2 at %d", node2Index1, node2Index2)
	}
}
