package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func parseYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func TestNodeReference_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected NodeReference
		wantErr  bool
	}{
		{
			name:     "single string",
			yaml:     "node: local",
			expected: NodeReference{"local"},
			wantErr:  false,
		},
		{
			name:     "array of strings",
			yaml:     "node: [node01, node02]",
			expected: NodeReference{"node01", "node02"},
			wantErr:  false,
		},
		{
			name:     "array with one string",
			yaml:     "node: [local]",
			expected: NodeReference{"local"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config struct {
				Node NodeReference `yaml:"node"`
			}

			err := parseYAML([]byte(tt.yaml), &config)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(config.Node) != len(tt.expected) {
					t.Errorf("NodeReference length = %v, want %v", len(config.Node), len(tt.expected))
					return
				}
				for i := range config.Node {
					if config.Node[i] != tt.expected[i] {
						t.Errorf("NodeReference[%d] = %v, want %v", i, config.Node[i], tt.expected[i])
					}
				}
			}
		})
	}
}

func TestExpandStepConfigs(t *testing.T) {
	tests := []struct {
		name     string
		configs  []*StepConfig
		expected int // expected number of configs after expansion
	}{
		{
			name: "single node",
			configs: []*StepConfig{
				{
					Name: "test",
					Node: NodeReference{"local"},
					Step: StepDetails{Type: "execute"},
				},
			},
			expected: 1,
		},
		{
			name: "multiple nodes",
			configs: []*StepConfig{
				{
					Name: "test",
					Node: NodeReference{"node01", "node02"},
					Step: StepDetails{Type: "execute"},
				},
			},
			expected: 2,
		},
		{
			name: "mixed single and multiple nodes",
			configs: []*StepConfig{
				{
					Name: "test1",
					Node: NodeReference{"local"},
					Step: StepDetails{Type: "execute"},
				},
				{
					Name: "test2",
					Node: NodeReference{"node01", "node02", "node03"},
					Step: StepDetails{Type: "execute"},
				},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandStepConfigs(tt.configs)
			if len(result) != tt.expected {
				t.Errorf("expandStepConfigs() got %v configs, want %v", len(result), tt.expected)
			}

			// Verify each expanded config has exactly one node
			for i, cfg := range result {
				if len(cfg.Node) != 1 {
					t.Errorf("expanded config[%d] has %d nodes, want 1", i, len(cfg.Node))
				}
			}
		})
	}
}

func TestExpandTestConfigs(t *testing.T) {
	tests := []struct {
		name     string
		configs  []*TestConfig
		expected int // expected number of configs after expansion
	}{
		{
			name: "single node",
			configs: []*TestConfig{
				{
					Name: "test",
					Node: NodeReference{"local"},
					Type: "execute",
				},
			},
			expected: 1,
		},
		{
			name: "multiple nodes",
			configs: []*TestConfig{
				{
					Name: "test",
					Node: NodeReference{"node01", "node02"},
					Type: "execute",
				},
			},
			expected: 2,
		},
		{
			name: "mixed single and multiple nodes",
			configs: []*TestConfig{
				{
					Name: "test1",
					Node: NodeReference{"local"},
					Type: "execute",
				},
				{
					Name: "test2",
					Node: NodeReference{"node01", "node02", "node03"},
					Type: "execute",
				},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTestConfigs(tt.configs)
			if len(result) != tt.expected {
				t.Errorf("expandTestConfigs() got %v configs, want %v", len(result), tt.expected)
			}

			// Verify each expanded config has exactly one node
			for i, cfg := range result {
				if len(cfg.Node) != 1 {
					t.Errorf("expanded config[%d] has %d nodes, want 1", i, len(cfg.Node))
				}
			}
		})
	}
}

func TestParseConfiguration_MultiNodeExpansion(t *testing.T) {
	yamlData := `
suite: Test Suite
nodes:
  - name: local
    type: local
    options:
      shell: /bin/bash
setup:
  - name: multi-node setup
    node: [local]
    step:
      type: simulated
      options:
        time: 1
tests:
  - name: single test
    node: local
    type: execute
    options:
      command: echo test
`

	config, err := ParseConfiguration([]byte(yamlData), ".")
	if err != nil {
		t.Fatalf("ParseConfiguration() error = %v", err)
	}

	if len(config.Setup) != 1 {
		t.Errorf("Setup should have 1 step, got %d", len(config.Setup))
	}

	if len(config.Tests) != 1 {
		t.Errorf("Tests should have 1 test, got %d", len(config.Tests))
	}
}
