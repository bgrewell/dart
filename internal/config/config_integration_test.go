package config

import (
	"testing"
)

// TestMultiNodeExpansionIntegration tests the complete flow of multi-node expansion
func TestMultiNodeExpansionIntegration(t *testing.T) {
	yamlData := `
suite: Integration Test Suite
nodes:
  - name: node01
    type: local
    options:
      shell: /bin/bash
  - name: node02
    type: local
    options:
      shell: /bin/bash
setup:
  # Single node step
  - name: single-node-step
    node: node01
    step:
      type: execute
      options:
        command: echo test
  # Multi-node step - should expand to 2 steps
  - name: multi-node-step
    node: [node01, node02]
    step:
      type: execute
      options:
        command: echo test
tests:
  # Single node test
  - name: single-node-test
    node: node01
    type: execute
    options:
      command: echo test
  # Multi-node test - should expand to 2 tests
  - name: multi-node-test
    node: [node01, node02]
    type: execute
    options:
      command: echo test
teardown:
  # Multi-node teardown - should expand to 2 steps
  - name: multi-node-teardown
    node: [node01, node02]
    step:
      type: execute
      options:
        command: echo cleanup
`
	
	config, err := ParseConfiguration([]byte(yamlData), ".")
	if err != nil {
		t.Fatalf("ParseConfiguration() error = %v", err)
	}
	
	// Verify nodes
	if len(config.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(config.Nodes))
	}
	
	// Verify setup expansion
	// 1 single-node + 2 from multi-node = 3 total
	if len(config.Setup) != 3 {
		t.Errorf("Setup should have 3 steps after expansion (1 single + 2 from multi-node), got %d", len(config.Setup))
	}
	
	// Verify all setup steps have exactly one node
	for i, step := range config.Setup {
		if len(step.Node) != 1 {
			t.Errorf("Setup step[%d] should have exactly 1 node after expansion, got %d", i, len(step.Node))
		}
	}
	
	// Verify tests expansion
	// 1 single-node + 2 from multi-node = 3 total
	if len(config.Tests) != 3 {
		t.Errorf("Tests should have 3 tests after expansion (1 single + 2 from multi-node), got %d", len(config.Tests))
	}
	
	// Verify all tests have exactly one node
	for i, test := range config.Tests {
		if len(test.Node) != 1 {
			t.Errorf("Test[%d] should have exactly 1 node after expansion, got %d", i, len(test.Node))
		}
	}
	
	// Verify teardown expansion
	// 2 from multi-node = 2 total
	if len(config.Teardown) != 2 {
		t.Errorf("Teardown should have 2 steps after expansion, got %d", len(config.Teardown))
	}
	
	// Verify all teardown steps have exactly one node
	for i, step := range config.Teardown {
		if len(step.Node) != 1 {
			t.Errorf("Teardown step[%d] should have exactly 1 node after expansion, got %d", i, len(step.Node))
		}
	}
	
	// Verify the node names in the expanded setup
	expectedSetupNodes := []string{"node01", "node01", "node02"}
	for i, step := range config.Setup {
		if step.Node[0] != expectedSetupNodes[i] {
			t.Errorf("Setup step[%d] should target node %s, got %s", i, expectedSetupNodes[i], step.Node[0])
		}
	}
	
	// Verify the node names in the expanded tests
	expectedTestNodes := []string{"node01", "node01", "node02"}
	for i, test := range config.Tests {
		if test.Node[0] != expectedTestNodes[i] {
			t.Errorf("Test[%d] should target node %s, got %s", i, expectedTestNodes[i], test.Node[0])
		}
	}
}

// TestMultiNodeWithThreeNodes tests expansion with three nodes
func TestMultiNodeWithThreeNodes(t *testing.T) {
	yamlData := `
suite: Three Node Test
nodes:
  - name: web01
    type: local
  - name: web02
    type: local
  - name: web03
    type: local
setup:
  - name: install-on-all
    node: [web01, web02, web03]
    step:
      type: simulated
      options:
        time: 1
`
	
	config, err := ParseConfiguration([]byte(yamlData), ".")
	if err != nil {
		t.Fatalf("ParseConfiguration() error = %v", err)
	}
	
	// Should expand to 3 steps
	if len(config.Setup) != 3 {
		t.Errorf("Setup should have 3 steps after expansion, got %d", len(config.Setup))
	}
	
	// Verify each targets the correct node
	expectedNodes := []string{"web01", "web02", "web03"}
	for i, step := range config.Setup {
		if step.Node[0] != expectedNodes[i] {
			t.Errorf("Step[%d] should target %s, got %s", i, expectedNodes[i], step.Node[0])
		}
		if step.Name != "install-on-all" {
			t.Errorf("Step[%d] should have name 'install-on-all', got '%s'", i, step.Name)
		}
	}
}

// TestEmptyNodeArray tests that empty arrays are handled properly
func TestEmptyNodeArray(t *testing.T) {
	yamlData := `
suite: Empty Array Test
nodes:
  - name: local
    type: local
setup:
  - name: test-step
    node: []
    step:
      type: simulated
      options:
        time: 1
`
	
	config, err := ParseConfiguration([]byte(yamlData), ".")
	if err != nil {
		t.Fatalf("ParseConfiguration() error = %v", err)
	}
	
	// Empty array should result in no steps after expansion
	if len(config.Setup) != 0 {
		t.Errorf("Setup should have 0 steps after expansion of empty array, got %d", len(config.Setup))
	}
}
