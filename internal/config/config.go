package config

import (
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"strings"
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

// DockerConfig is the configuration for Docker
type DockerConfig struct {
	Networks []*NetworkConfig `json:"networks" yaml:"networks"`
	Images   []*ImageConfig   `json:"images" yaml:"images"`
}

// NetworkConfig is the configuration for a single network
type NetworkConfig struct {
	Name    string `json:"name" yaml:"name"`
	Subnet  string `json:"subnet" yaml:"subnet"`
	Gateway string `json:"gateway" yaml:"gateway"`
}

// ImageConfig is the configuration for a single image
type ImageConfig struct {
	Name       string `json:"name" yaml:"name"`
	Tag        string `json:"tag" yaml:"tag"`
	Dockerfile string `json:"dockerfile" yaml:"dockerfile"`
}

// Configuration is the top-level configuration for the test suite
type Configuration struct {
	Suite    string        `json:"suite" yaml:"suite"`
	Docker   *DockerConfig `json:"docker" yaml:"docker"`
	Setup    []*StepConfig `json:"setup" yaml:"setup"`
	Nodes    []*NodeConfig `json:"nodes" yaml:"nodes"`
	Tests    []*TestConfig `json:"tests" yaml:"tests"`
	Teardown []*StepConfig `json:"teardown" yaml:"teardown"`
}

func LoadConfiguration(cfgPath string) (config *Configuration, err error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	dir := path.Dir(cfgPath)

	return ParseConfiguration(data, dir)
}

func ParseConfiguration(data []byte, location string) (config *Configuration, err error) {
	config = &Configuration{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	for i, test := range config.Tests {
		test.Order = i
	}

	// Ensure that the Dockerfile paths that are relative to the execution point
	if config.Docker != nil {
		for _, image := range config.Docker.Images {
			if !strings.HasPrefix(image.Dockerfile, "/") {
				image.Dockerfile = path.Join(location, image.Dockerfile)
			}
		}
	}

	return config, nil
}
