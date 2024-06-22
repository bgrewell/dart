package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

// StepConfig is the configuration for a single setup/teardown step
type StepConfig struct {
	Name string      `json:"name" yaml:"name"`
	Node string      `json:"node" yaml:"node"`
	Step StepDetails `json:"step" yaml:"step"`
}

type StepDetails struct {
	Type    string                 `json:"type" yaml:"type"`
	Options map[string]interface{} `json:"options" yaml:"options"`
}

// NodeConfig is the configuration for a single node
type NodeConfig struct {
	Name    string                 `json:"name" yaml:"name"`
	Type    string                 `json:"type" yaml:"type"`
	Options map[string]interface{} `json:"options" yaml:"options"`
}

// TestConfig is the configuration for a single test
type TestConfig struct {
	Order       int                    `json:"-" yaml:"-"`
	Name        string                 `json:"name" yaml:"name"`
	Node        string                 `json:"node" yaml:"node"`
	PreExecute  []string               `json:"preExecute" yaml:"preExecute"`
	Execute     string                 `json:"execute" yaml:"execute"`
	PostExecute []string               `json:"postExecute" yaml:"postExecute"`
	Check       map[string]interface{} `json:"check" yaml:"check"`
}

// Configuration is the top-level configuration for the test suite
type Configuration struct {
	Suite    string       `json:"suite" yaml:"suite"`
	Setup    []StepConfig `json:"setup" yaml:"setup"`
	Nodes    []NodeConfig `json:"nodes" yaml:"nodes"`
	Tests    []TestConfig `json:"tests" yaml:"tests"`
	Teardown []StepConfig `json:"teardown" yaml:"teardown"`
}

func LoadConfiguration(path string) (config *Configuration, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfiguration(data)
}

func ParseConfiguration(data []byte) (config *Configuration, err error) {
	config = &Configuration{}
	err = yaml.Unmarshal(data, config)

	for i, test := range config.Tests {
		test.Order = i
	}
	return
}
