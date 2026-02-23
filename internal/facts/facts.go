package facts

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// FactStore holds gathered facts indexed by node name then fact name.
type FactStore map[string]map[string]string

// GatherFacts executes fact commands on each node that has facts defined and
// returns a populated FactStore. Commands are run via the node's Execute method
// and must exit with code 0. Trailing whitespace is trimmed from output.
func GatherFacts(nodes map[string]ifaces.Node, configs []*config.NodeConfig) (FactStore, error) {
	store := make(FactStore)

	for _, cfg := range configs {
		if len(cfg.Facts) == 0 {
			continue
		}

		node, ok := nodes[cfg.Name]
		if !ok {
			return nil, fmt.Errorf("node %q not found while gathering facts", cfg.Name)
		}

		nodeFacts := make(map[string]string, len(cfg.Facts))
		for name, command := range cfg.Facts {
			result, err := node.Execute(command)
			if err != nil {
				return nil, fmt.Errorf("fact %q on node %q failed: %w", name, cfg.Name, err)
			}
			if result.ExitCode != 0 {
				var stderr string
				if result.Stderr != nil {
					buf := new(bytes.Buffer)
					buf.ReadFrom(result.Stderr)
					stderr = buf.String()
				}
				return nil, fmt.Errorf("fact %q on node %q exited with code %d: %s", name, cfg.Name, result.ExitCode, strings.TrimSpace(stderr))
			}

			var stdout string
			if result.Stdout != nil {
				buf := new(bytes.Buffer)
				buf.ReadFrom(result.Stdout)
				stdout = buf.String()
			}
			nodeFacts[name] = strings.TrimRight(stdout, " \t\r\n")
		}
		store[cfg.Name] = nodeFacts
	}

	return store, nil
}

// RenderTemplate processes a single string through text/template with a
// fact(nodeName, factName) function. If the string contains no template
// delimiters it is returned unchanged.
func RenderTemplate(text string, store FactStore, currentNode string) (string, error) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}

	funcMap := template.FuncMap{
		"fact": func(nodeName, factName string) (string, error) {
			if nodeName == "self" {
				nodeName = currentNode
			}
			nodeFacts, ok := store[nodeName]
			if !ok {
				return "", fmt.Errorf("no facts for node %q", nodeName)
			}
			value, ok := nodeFacts[factName]
			if !ok {
				return "", fmt.Errorf("fact %q not found on node %q", factName, nodeName)
			}
			return value, nil
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).Parse(text)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// ProcessConfigOptions recursively walks a map[string]interface{} and renders
// all string values through RenderTemplate. Nested maps and slices are handled.
func ProcessConfigOptions(opts map[string]interface{}, store FactStore, currentNode string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(opts))
	for k, v := range opts {
		processed, err := processValue(v, store, currentNode)
		if err != nil {
			return nil, err
		}
		result[k] = processed
	}
	return result, nil
}

func processValue(v interface{}, store FactStore, currentNode string) (interface{}, error) {
	switch val := v.(type) {
	case string:
		return RenderTemplate(val, store, currentNode)
	case map[string]interface{}:
		return ProcessConfigOptions(val, store, currentNode)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			processed, err := processValue(item, store, currentNode)
			if err != nil {
				return nil, err
			}
			result[i] = processed
		}
		return result, nil
	default:
		return v, nil
	}
}

// ProcessStepConfigs renders templates in step config options and commands.
func ProcessStepConfigs(configs []*config.StepConfig, store FactStore) ([]*config.StepConfig, error) {
	for _, cfg := range configs {
		currentNode := cfg.Node[0]

		processed, err := ProcessConfigOptions(cfg.Step.Options, store, currentNode)
		if err != nil {
			return nil, fmt.Errorf("step %q: %w", cfg.Name, err)
		}
		cfg.Step.Options = processed
	}
	return configs, nil
}

// ProcessTestConfigs renders templates in test config options, commands,
// and pre/post test commands (Setup/Teardown string slices).
func ProcessTestConfigs(configs []*config.TestConfig, store FactStore) ([]*config.TestConfig, error) {
	for _, cfg := range configs {
		currentNode := cfg.Node[0]

		processed, err := ProcessConfigOptions(cfg.Options, store, currentNode)
		if err != nil {
			return nil, fmt.Errorf("test %q: %w", cfg.Name, err)
		}
		cfg.Options = processed

		for i, cmd := range cfg.Setup {
			rendered, err := RenderTemplate(cmd, store, currentNode)
			if err != nil {
				return nil, fmt.Errorf("test %q setup command: %w", cfg.Name, err)
			}
			cfg.Setup[i] = rendered
		}

		for i, cmd := range cfg.Teardown {
			rendered, err := RenderTemplate(cmd, store, currentNode)
			if err != nil {
				return nil, fmt.Errorf("test %q teardown command: %w", cfg.Name, err)
			}
			cfg.Teardown[i] = rendered
		}
	}
	return configs, nil
}

// HasFacts returns true if any node config has facts defined.
func HasFacts(configs []*config.NodeConfig) bool {
	for _, cfg := range configs {
		if len(cfg.Facts) > 0 {
			return true
		}
	}
	return false
}
