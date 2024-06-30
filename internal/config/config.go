package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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
	data, err = processLoadFromDirectives(data, location)
	if err != nil {
		return nil, err
	}

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

func processLoadFromDirectives(data []byte, location string) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var outputLines []string

	for _, line := range lines {
		if strings.Contains(line, "!!load_from(") {
			startIdx := strings.Index(line, "!!load_from(") + len("!!load_from(")
			endIdx := strings.Index(line[startIdx:], ")")
			dir := line[startIdx : startIdx+endIdx]

			loadedData, err := loadFromDirectory(path.Join(location, dir))
			if err != nil {
				return nil, err
			}
			indentedLoadedData := indent(loadedData, "  ") // Indent the loaded data
			outputLines = append(outputLines, fmt.Sprintf("%s\n%s", line[:startIdx-len("!!load_from(")], indentedLoadedData))
		} else {
			outputLines = append(outputLines, line)
		}
	}

	return []byte(strings.Join(outputLines, "\n")), nil
}

func loadFromDirectory(dir string) ([]byte, error) {
	var buffer bytes.Buffer

	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			buffer.Write(data)
			buffer.WriteString("\n")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func indent(data []byte, prefix string) string {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
