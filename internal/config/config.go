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

// NodeReference can be either a single node name (string) or multiple node names ([]string)
type NodeReference []string

// UnmarshalYAML implements custom unmarshaling for NodeReference
// It accepts either a single string or an array of strings
func (n *NodeReference) UnmarshalYAML(value *yaml.Node) error {
	// Try unmarshaling as a string first
	var single string
	if err := value.Decode(&single); err == nil {
		*n = NodeReference{single}
		return nil
	}

	// Try unmarshaling as an array of strings
	var multiple []string
	if err := value.Decode(&multiple); err == nil {
		*n = NodeReference(multiple)
		return nil
	}

	return fmt.Errorf("node must be a string or array of strings")
}

// MarshalYAML implements custom marshaling for NodeReference
// If there's only one node, marshal as a string; otherwise as an array
func (n NodeReference) MarshalYAML() (interface{}, error) {
	if len(n) == 1 {
		return n[0], nil
	}
	return []string(n), nil
}

// Configuration is the top-level configuration for the test suite
type Configuration struct {
	Suite    string        `json:"suite" yaml:"suite"`
	Docker   *DockerConfig `json:"docker" yaml:"docker"`
	Lxd      *LxdConfig    `json:"lxd" yaml:"lxd"`
	Setup    []*StepConfig `json:"setup" yaml:"setup"`
	Teardown []*StepConfig `json:"teardown" yaml:"teardown"`
	Nodes    []*NodeConfig `json:"nodes" yaml:"nodes"`
	Tests    []*TestConfig `json:"tests" yaml:"tests"`
}

// DockerConfig is the configuration for Docker
type DockerConfig struct {
	Networks []*NetworkConfig `json:"networks" yaml:"networks"`
	Images   []*ImageConfig   `json:"images" yaml:"images"`
}

// LxdConfig is the configuration for LXD
type LxdConfig struct {
	Socket   string              `json:"socket" yaml:"socket"`       // Unix socket path for local connections
	Project  *LxdProjectConfig   `json:"project" yaml:"project"`
	Networks []*LxdNetworkConfig `json:"networks" yaml:"networks"`
	Profiles []*LxdProfileConfig `json:"profiles" yaml:"profiles"`
	Images   []*LxdImageConfig   `json:"images" yaml:"images"`
}

// StepConfig is the configuration for a single setup/teardown step
type StepConfig struct {
	Name string        `json:"name" yaml:"name"`
	Node NodeReference `json:"node" yaml:"node"`
	Step StepDetails   `json:"step" yaml:"step"`
}

// StepDetails is the details of a single step
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
	Order    int                    `json:"-" yaml:"-"`
	Name     string                 `json:"name" yaml:"name"`
	Node     NodeReference          `json:"node" yaml:"node"`
	Setup    []string               `json:"setup" yaml:"setup"`
	Teardown []string               `json:"teardown" yaml:"teardown"`
	Type     string                 `json:"type" yaml:"type"`
	Options  map[string]interface{} `json:"options" yaml:"options"`
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

// SudoConfig is the configuration for sudo abilities on a node
type SudoConfig struct {
	Password    string `json:"password" yaml:"password"`
	EnvVar      string `json:"env_var" yaml:"env_var"`
	VaultSecret string `json:"vault_secret" yaml:"vault_secret"`
}

// LxdNetworkConfig is the configuration for an LXD network
type LxdNetworkConfig struct {
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"` // "bridge", "ovn", etc.
	Subnet  string `json:"subnet" yaml:"subnet"`
	Gateway string `json:"gateway" yaml:"gateway"`
}

// LxdProfileConfig is the configuration for an LXD profile
type LxdProfileConfig struct {
	Name        string                       `json:"name" yaml:"name"`
	Description string                       `json:"description" yaml:"description"`
	Config      map[string]string            `json:"config" yaml:"config"`
	Devices     map[string]*LxdDeviceConfig  `json:"devices" yaml:"devices"`
}

// LxdDeviceConfig is the configuration for an LXD device
type LxdDeviceConfig struct {
	Type string            `json:"type" yaml:"type"` // "disk", "nic", "unix-char", etc.
	Path string            `json:"path,omitempty" yaml:"path,omitempty"`
	Pool string            `json:"pool,omitempty" yaml:"pool,omitempty"`
	Name string            `json:"name,omitempty" yaml:"name,omitempty"`
	Opts map[string]string `json:"opts,omitempty" yaml:"opts,omitempty"`
}

// LxdImageConfig is the configuration for an LXD image
type LxdImageConfig struct {
	Alias    string `json:"alias" yaml:"alias"`
	Server   string `json:"server" yaml:"server"`
	Protocol string `json:"protocol" yaml:"protocol"` // "lxd" or "simplestreams"
}

// LxdProjectConfig is the configuration for an LXD project
type LxdProjectConfig struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Config      map[string]string `json:"config" yaml:"config"`
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

	// Expand multi-node configurations
	config.Setup = expandStepConfigs(config.Setup)
	config.Teardown = expandStepConfigs(config.Teardown)
	config.Tests = expandTestConfigs(config.Tests)

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

// expandStepConfigs expands step configurations with multiple nodes into individual step configs
func expandStepConfigs(configs []*StepConfig) []*StepConfig {
	var expanded []*StepConfig
	for _, cfg := range configs {
		if len(cfg.Node) == 1 {
			// Single node - keep as is
			expanded = append(expanded, cfg)
		} else {
			// Multiple nodes - create a copy for each node
			for _, nodeName := range cfg.Node {
				// Create a copy of the config
				newCfg := &StepConfig{
					Name: cfg.Name,
					Node: NodeReference{nodeName},
					Step: cfg.Step,
				}
				expanded = append(expanded, newCfg)
			}
		}
	}
	return expanded
}

// expandTestConfigs expands test configurations with multiple nodes into individual test configs
func expandTestConfigs(configs []*TestConfig) []*TestConfig {
	var expanded []*TestConfig
	for _, cfg := range configs {
		if len(cfg.Node) == 1 {
			// Single node - keep as is
			expanded = append(expanded, cfg)
		} else {
			// Multiple nodes - create a copy for each node
			for _, nodeName := range cfg.Node {
				// Create a copy of the config
				newCfg := &TestConfig{
					Order:    cfg.Order,
					Name:     cfg.Name,
					Node:     NodeReference{nodeName},
					Setup:    cfg.Setup,
					Teardown: cfg.Teardown,
					Type:     cfg.Type,
					Options:  cfg.Options,
				}
				expanded = append(expanded, newCfg)
			}
		}
	}
	return expanded
}
