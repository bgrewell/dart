package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	Suite    string            `json:"suite" yaml:"suite"`
	Vars     map[string]string `json:"vars" yaml:"vars"`
	Docker   *DockerConfig     `json:"docker" yaml:"docker"`
	Lxd      *LxdConfig        `json:"lxd" yaml:"lxd"`
	Setup    []*StepConfig     `json:"setup" yaml:"setup"`
	Teardown []*StepConfig     `json:"teardown" yaml:"teardown"`
	Nodes    []*NodeConfig     `json:"nodes" yaml:"nodes"`
	Tests    []*TestConfig     `json:"tests" yaml:"tests"`
}

// DockerConfig is the configuration for Docker
type DockerConfig struct {
	Networks []*NetworkConfig `json:"networks" yaml:"networks"`
	Images   []*ImageConfig   `json:"images" yaml:"images"`
}

// LxdConfig is the configuration for LXD
type LxdConfig struct {
	Socket   string              `json:"socket" yaml:"socket"` // Unix socket path for local connections
	Project  *LxdProjectConfig   `json:"project" yaml:"project"`
	Networks []*LxdNetworkConfig `json:"networks" yaml:"networks"`
	Profiles []*LxdProfileConfig `json:"profiles" yaml:"profiles"`
	Images   []*LxdImageConfig   `json:"images" yaml:"images"`
}

// StepConfig is the configuration for a single setup/teardown step
type StepConfig struct {
	Name    string         `json:"name" yaml:"name"`
	Node    NodeReference  `json:"node" yaml:"node"`
	Step    StepDetails    `json:"step" yaml:"step"`
	Loc     SourceLocation `json:"-" yaml:"-"`
	NodeLoc SourceLocation `json:"-" yaml:"-"`
}

// StepDetails is the details of a single step
type StepDetails struct {
	Type    string                 `json:"type" yaml:"type"`
	Options map[string]interface{} `json:"options" yaml:"options"`
	TypeLoc SourceLocation         `json:"-" yaml:"-"`
}

// NodeConfig is the configuration for a single node
type NodeConfig struct {
	Name    string                 `json:"name" yaml:"name"`
	Type    string                 `json:"type" yaml:"type"`
	Options map[string]interface{} `json:"options" yaml:"options"`
	Facts   map[string]string      `json:"facts,omitempty" yaml:"facts,omitempty"`
	Loc     SourceLocation         `json:"-" yaml:"-"`
	TypeLoc SourceLocation         `json:"-" yaml:"-"`
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
	// SkipIf/SkipUnless are commands run on the test's node before the
	// test; SkipIf skips when the command succeeds, SkipUnless skips when
	// it fails. Skipped tests report a distinct Skip status.
	SkipIf     string `json:"skip_if" yaml:"skip_if"`
	SkipUnless string `json:"skip_unless" yaml:"skip_unless"`
	// Retry reruns the test (command + evaluations) until it passes or the
	// timeout elapses — for eventually-consistent assertions.
	Retry *RetryConfig `json:"retry,omitempty" yaml:"retry,omitempty"`
	// Tags label the test for --only/--skip filtering.
	Tags    []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	Loc     SourceLocation `json:"-" yaml:"-"`
	NodeLoc SourceLocation `json:"-" yaml:"-"`
	TypeLoc SourceLocation `json:"-" yaml:"-"`
}

// RetryConfig configures eventually-consistent retry for a test.
type RetryConfig struct {
	Timeout  float64 `json:"timeout" yaml:"timeout"`   // Seconds to keep retrying (required, > 0)
	Interval float64 `json:"interval" yaml:"interval"` // Seconds between attempts (default 2)
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
	// Nat controls ipv4.nat on the bridge. Defaults to true when omitted;
	// set to false for air-gapped networks where instances must not reach
	// the internet from the moment they boot.
	Nat *bool `json:"nat,omitempty" yaml:"nat,omitempty"`
}

// LxdProfileConfig is the configuration for an LXD profile
type LxdProfileConfig struct {
	Name        string                      `json:"name" yaml:"name"`
	Description string                      `json:"description" yaml:"description"`
	Config      map[string]string           `json:"config" yaml:"config"`
	Devices     map[string]*LxdDeviceConfig `json:"devices" yaml:"devices"`
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
	return LoadConfigurationWithVars(cfgPath, nil)
}

// LoadConfigurationWithVars loads a configuration with CLI variable
// overrides applied on top of the suite's vars block.
func LoadConfigurationWithVars(cfgPath string, cliVars map[string]string) (config *Configuration, err error) {
	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		absPath = cfgPath
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(absPath)

	return ParseConfigurationWithVars(data, dir, cliVars, absPath)
}

func ParseConfiguration(data []byte, location string, filePath ...string) (config *Configuration, err error) {
	return ParseConfigurationWithVars(data, location, nil, filePath...)
}

// ParseConfigurationWithVars parses a configuration after substituting
// {{var.name}} and {{env.NAME}} references. Variables come from the
// suite's vars block, overridden by cliVars (--var on the command line).
func ParseConfigurationWithVars(data []byte, location string, cliVars map[string]string, filePath ...string) (config *Configuration, err error) {
	processed, usedLoadFrom, err := processLoadFromDirectives(data, location)
	if err != nil {
		return nil, err
	}
	data = processed

	data, err = substituteVars(data, cliVars)
	if err != nil {
		return nil, err
	}

	config = &Configuration{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	// Extract line numbers before expansion (indices match 1:1 with YAML
	// sequences). Skipped when load_from inlined other files: the processed
	// buffer's line numbers no longer correspond to the file on disk, and a
	// snippet pointing at the wrong line is worse than none.
	if len(filePath) > 0 && filePath[0] != "" && !usedLoadFrom {
		extractLocations(data, filePath[0], config)
	}

	if err := validateConfiguration(config); err != nil {
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
			if !filepath.IsAbs(image.Dockerfile) {
				image.Dockerfile = filepath.Join(location, image.Dockerfile)
			}
		}
	}

	return config, nil
}

var varRefRe = regexp.MustCompile(`\{\{\s*(var|env)\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// substituteVars replaces {{var.name}} and {{env.NAME}} references in the
// raw YAML before unmarshaling, so substituted values land with native
// YAML types (a numeric var in a numeric position stays a number). The
// substitution is inline and single-line, so source locations survive.
// Values may not contain newlines (they would corrupt YAML structure and
// shift line numbers). Var values may themselves reference {{env.NAME}};
// deeper nesting is not supported. Capture references and fact templates
// use different prefixes and pass through untouched.
func substituteVars(data []byte, cliVars map[string]string) ([]byte, error) {
	// The vars block is read pre-substitution; CLI overrides win
	var head struct {
		Vars map[string]string `yaml:"vars"`
	}
	_ = yaml.Unmarshal(data, &head) // full parse later reports YAML errors
	vars := make(map[string]string, len(head.Vars)+len(cliVars))
	for k, v := range head.Vars {
		vars[k] = v
	}
	for k, v := range cliVars {
		vars[k] = v
	}

	var missing []string
	resolveEnv := func(text string) string {
		return varRefRe.ReplaceAllStringFunc(text, func(ref string) string {
			m := varRefRe.FindStringSubmatch(ref)
			if m[1] != "env" {
				return ref
			}
			if value, ok := os.LookupEnv(m[2]); ok {
				return value
			}
			missing = append(missing, "env."+m[2])
			return ref
		})
	}

	// Resolve env references inside var values first, so a var defined as
	// "{{env.HOME}}/data" substitutes fully resolved
	for name, value := range vars {
		vars[name] = resolveEnv(value)
	}
	for name, value := range vars {
		if strings.ContainsAny(value, "\n\r") {
			return nil, fmt.Errorf("var %q contains a newline; variable values must be single-line", name)
		}
		_ = value
	}

	result := varRefRe.ReplaceAllFunc(data, func(ref []byte) []byte {
		m := varRefRe.FindSubmatch(ref)
		kind, name := string(m[1]), string(m[2])
		if kind == "var" {
			if value, ok := vars[name]; ok {
				return []byte(value)
			}
			missing = append(missing, "var."+name)
			return ref
		}
		if value, ok := os.LookupEnv(name); ok {
			if strings.ContainsAny(value, "\n\r") {
				missing = append(missing, fmt.Sprintf("env.%s (value contains a newline)", name))
				return ref
			}
			return []byte(value)
		}
		missing = append(missing, "env."+name)
		return ref
	})

	if len(missing) > 0 {
		slices.Sort(missing)
		missing = slices.Compact(missing)
		return nil, fmt.Errorf("unresolved references: %s (define in the vars block, pass --vars, or set the environment variable)", strings.Join(missing, ", "))
	}
	return result, nil
}

// validateConfiguration rejects configurations that would otherwise fail
// obscurely (or worse, silently) later: duplicate or unnamed nodes, and
// steps/tests that reference no node. A step or test with a missing or
// empty `node:` used to be silently dropped during expansion — a suite
// could pass with fewer tests than written, which is the same
// false-positive class as skips rendering as passes.
func validateConfiguration(cfg *Configuration) error {
	seen := make(map[string]bool, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		if node.Name == "" {
			return &ConfigError{Message: "node has no name", Location: node.Loc}
		}
		if seen[node.Name] {
			return &ConfigError{
				Message:  fmt.Sprintf("duplicate node name %q", node.Name),
				Location: node.Loc,
			}
		}
		seen[node.Name] = true
	}

	for _, step := range cfg.Setup {
		if len(step.Node) == 0 {
			return &ConfigError{
				Message:  fmt.Sprintf("setup step %q references no node", step.Name),
				Location: step.Loc,
			}
		}
	}
	for _, step := range cfg.Teardown {
		if len(step.Node) == 0 {
			return &ConfigError{
				Message:  fmt.Sprintf("teardown step %q references no node", step.Name),
				Location: step.Loc,
			}
		}
	}
	for _, test := range cfg.Tests {
		if len(test.Node) == 0 {
			return &ConfigError{
				Message:  fmt.Sprintf("test %q references no node", test.Name),
				Location: test.Loc,
			}
		}
	}
	return nil
}

func processLoadFromDirectives(data []byte, location string) (processed []byte, usedLoadFrom bool, err error) {
	lines := strings.Split(string(data), "\n")
	var outputLines []string

	for lineNum, line := range lines {
		if strings.Contains(line, "!!load_from(") {
			startIdx := strings.Index(line, "!!load_from(") + len("!!load_from(")
			endIdx := strings.Index(line[startIdx:], ")")
			if endIdx < 0 {
				return nil, false, fmt.Errorf("malformed !!load_from directive on line %d: missing closing parenthesis", lineNum+1)
			}
			dir := line[startIdx : startIdx+endIdx]

			loadedData, err := loadFromDirectory(filepath.Join(location, dir))
			if err != nil {
				return nil, false, err
			}
			usedLoadFrom = true
			indentedLoadedData := indent(loadedData, "  ") // Indent the loaded data
			outputLines = append(outputLines, fmt.Sprintf("%s\n%s", line[:startIdx-len("!!load_from(")], indentedLoadedData))
		} else {
			outputLines = append(outputLines, line)
		}
	}

	return []byte(strings.Join(outputLines, "\n")), usedLoadFrom, nil
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
					Name:    cfg.Name,
					Node:    NodeReference{nodeName},
					Step:    cfg.Step,
					Loc:     cfg.Loc,
					NodeLoc: cfg.NodeLoc,
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
			// Multiple nodes - create a copy for each node. Setup/Teardown
			// slices are cloned: fact-template rendering rewrites their
			// entries in place per node, so sharing the backing array would
			// leak one node's rendered values into its siblings.
			for _, nodeName := range cfg.Node {
				newCfg := &TestConfig{
					Order:      cfg.Order,
					Name:       cfg.Name,
					Node:       NodeReference{nodeName},
					Setup:      slices.Clone(cfg.Setup),
					Teardown:   slices.Clone(cfg.Teardown),
					Type:       cfg.Type,
					Options:    cfg.Options,
					SkipIf:     cfg.SkipIf,
					SkipUnless: cfg.SkipUnless,
					Retry:      cfg.Retry,
					Tags:       slices.Clone(cfg.Tags),
					Loc:        cfg.Loc,
					NodeLoc:    cfg.NodeLoc,
					TypeLoc:    cfg.TypeLoc,
				}
				expanded = append(expanded, newCfg)
			}
		}
	}
	return expanded
}
