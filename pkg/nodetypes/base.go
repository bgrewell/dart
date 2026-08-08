package nodetypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/pkg/ifaces"
)

type BaseNode struct {
}

// knownNodeTypes mirrors the factory switch in CreateNodesWithWrappers;
// IsKnownNodeType lets validate-only paths (--check) reject unknown types
// without constructing real nodes.
var knownNodeTypes = map[string]bool{
	"local": true, "docker": true, "docker-compose": true,
	"ssh": true, "lxd": true, "lxd-vm": true,
}

// IsKnownNodeType reports whether the factory can construct this type.
func IsKnownNodeType(nodeType string) bool {
	return knownNodeTypes[nodeType]
}

// ValidateNodeOptions checks everything about a single node that can be
// verified without contacting anything, so --check catches it before a run:
// required fields, unreadable known_hosts, missing SSH credentials, and
// malformed bastion, volume, or port specifications.
func ValidateNodeOptions(cfg *config.NodeConfig) error {
	if err := validateOptionNames(cfg); err != nil {
		return err
	}

	switch cfg.Type {
	case "ssh":
		var opts SshNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		// Without a host the dial goes to ":22" and fails mid-run with a
		// connection error that says nothing about the real mistake
		if opts.Host == "" {
			return fmt.Errorf("host is required")
		}
		if _, err := sshAuthMethods(opts.KeyFile, opts.Pass); err != nil {
			return err
		}
		if _, err := hostKeyCallbackFor(opts.KnownHosts, opts.InsecureSkipHostKey); err != nil {
			return err
		}
		if opts.Bastion != nil {
			if opts.Bastion.Host == "" {
				return fmt.Errorf("bastion host is required")
			}
			if opts.Bastion.Bastion != nil {
				return fmt.Errorf("chained bastions are not supported: remove the nested bastion block")
			}
			if _, err := sshAuthMethods(opts.Bastion.KeyFile, opts.Bastion.Pass); err != nil {
				return fmt.Errorf("bastion: %w", err)
			}
		}
	case "docker":
		var opts DockerNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		if opts.Image == "" {
			return fmt.Errorf("image is required")
		}
		if _, err := resolveVolumes(opts.Volumes, cfg.SuiteDir); err != nil {
			return err
		}
		if len(opts.Ports) > 0 {
			if err := docker.ValidatePortSpecs(opts.Ports); err != nil {
				return err
			}
		}
	case "docker-compose":
		var composeOpts DockerComposeNodeOpts
		if err := decodeNodeOptions(cfg.Options, &composeOpts); err != nil {
			return err
		}
		if composeOpts.ComposeFile == "" {
			return fmt.Errorf("compose_file is required")
		}

		// A compose node also accepts the docker option shapes
		var opts DockerNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		if _, err := resolveVolumes(opts.Volumes, cfg.SuiteDir); err != nil {
			return err
		}
		if len(opts.Ports) > 0 {
			if err := docker.ValidatePortSpecs(opts.Ports); err != nil {
				return err
			}
		}
	case "lxd", "lxd-vm":
		var opts LxdNodeOpts
		if err := decodeNodeOptions(cfg.Options, &opts); err != nil {
			return err
		}
		if err := opts.validate(); err != nil {
			return err
		}
	}
	return nil
}

// optionKeysOf collects the option names a set of typed option structs
// accepts, read from their json tags — the same tags the decode uses, so
// the two cannot drift.
func optionKeysOf(targets ...interface{}) map[string]bool {
	keys := map[string]bool{}
	for _, target := range targets {
		t := reflect.TypeOf(target)
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		for i := 0; i < t.NumField(); i++ {
			tag := t.Field(i).Tag.Get("json")
			name, _, _ := strings.Cut(tag, ",")
			if name != "" && name != "-" {
				keys[name] = true
			}
		}
	}
	return keys
}

// knownNodeOptions lists the option names each node type accepts. A key
// outside the set is a typo or a misplaced option: the decode silently
// drops it, so without this check the option reads as configured while
// nothing consumes it.
func knownNodeOptions(nodeType string) map[string]bool {
	switch nodeType {
	case "local":
		return map[string]bool{"env": true, "shell": true, "sudo": true, "exec_opts": true}
	case "ssh":
		return optionKeysOf(SshNodeOpts{})
	case "docker":
		return optionKeysOf(DockerNodeOpts{})
	case "docker-compose":
		return optionKeysOf(DockerComposeNodeOpts{}, DockerNodeOpts{})
	case "lxd", "lxd-vm":
		return optionKeysOf(LxdNodeOpts{})
	}
	return nil
}

// validateOptionNames reports the first unrecognized option name, listing
// what the type accepts so the fix is obvious.
func validateOptionNames(cfg *config.NodeConfig) error {
	known := knownNodeOptions(cfg.Type)
	if known == nil {
		return nil
	}

	unknown := make([]string, 0, len(cfg.Options))
	for key := range cfg.Options {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)

	accepted := make([]string, 0, len(known))
	for key := range known {
		accepted = append(accepted, key)
	}
	sort.Strings(accepted)

	return fmt.Errorf("unknown option %q for a %s node (accepted: %s)",
		unknown[0], cfg.Type, strings.Join(accepted, ", "))
}

// decodeNodeOptions reuses the same JSON round-trip the factories use, so
// validation sees exactly what construction would.
func decodeNodeOptions(options map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(options)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// CreateNodes creates nodes using only the Docker wrapper (backward compatible)
func CreateNodes(configs []*config.NodeConfig, wrapper *docker.Wrapper) (map[string]ifaces.Node, error) {
	return CreateNodesWithWrappers(configs, wrapper, nil)
}

// ValidateNodeSet checks the constraints that span the whole node list
// rather than a single node: names must be unique, and at most one node may
// be of type local. Keeping them here rather than inline in the factory is
// what lets --check report them without constructing anything.
func ValidateNodeSet(configs []*config.NodeConfig) error {
	seen := make(map[string]bool, len(configs))
	localNode := ""

	for _, cfg := range configs {
		if seen[cfg.Name] {
			return &config.ConfigError{
				Message:  fmt.Sprintf("duplicate node name %q", cfg.Name),
				Location: cfg.Loc,
			}
		}
		seen[cfg.Name] = true

		if cfg.Type == "local" {
			if localNode != "" {
				return &config.ConfigError{
					Message:  fmt.Sprintf("only one local node allowed; %q duplicates %q", cfg.Name, localNode),
					Location: cfg.Loc,
				}
			}
			localNode = cfg.Name
		}
	}
	return nil
}

// CreateNodesWithWrappers creates nodes using both Docker and LXD wrappers
func CreateNodesWithWrappers(configs []*config.NodeConfig, dockerWrapper *docker.Wrapper, lxdWrapper *lxd.Wrapper) (map[string]ifaces.Node, error) {
	if err := ValidateNodeSet(configs); err != nil {
		return nil, err
	}

	nodes := make(map[string]ifaces.Node)

	for _, cfg := range configs {
		var node ifaces.Node
		var err error

		switch cfg.Type {
		case "local":
			node = NewLocalNode(cfg.Name, &cfg.Options)
		case "docker":
			node, err = NewDockerNode(dockerWrapper, cfg.Name, &cfg.Options, cfg.SuiteDir)
		case "docker-compose":
			node, err = NewDockerComposeNode(dockerWrapper, cfg.Name, &cfg.Options, cfg.SuiteDir)
		case "ssh":
			node, err = NewSshNode(cfg.Name, &cfg.Options, cfg.SuiteDir)
		case "lxd":
			if lxdWrapper != nil {
				node, err = NewLxdNodeWithWrapper(lxdWrapper, cfg.Name, &cfg.Options, cfg.SuiteDir)
			} else {
				node, err = NewLxdNode(cfg.Name, &cfg.Options, cfg.SuiteDir)
			}
		case "lxd-vm":
			// Alias for LXD virtual machine type. The options map is copied
			// rather than mutated: cfg.Options belongs to the configuration.
			opts := make(map[string]interface{}, len(cfg.Options)+1)
			for k, v := range cfg.Options {
				opts[k] = v
			}
			opts["instance_type"] = "virtual-machine"
			if lxdWrapper != nil {
				node, err = NewLxdNodeWithWrapper(lxdWrapper, cfg.Name, &opts, cfg.SuiteDir)
			} else {
				node, err = NewLxdNode(cfg.Name, &opts, cfg.SuiteDir)
			}
		default:
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("unknown node type %q", cfg.Type),
				Location: cfg.TypeLoc,
			}
		}

		if err != nil {
			return nil, err
		}

		nodes[cfg.Name] = node
	}

	return nodes, nil
}
