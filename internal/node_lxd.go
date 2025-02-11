package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/internal/lxc"
	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"strings"
	"unicode"
)

var _ Node = &LxdNode{}

type LxdNetworkOpts struct {
	Name   string `yaml:"name,omitempty" json:"name"`
	Subnet string `yaml:"subnet,omitempty" json:"subnet"`
	Ip     string `yaml:"ip,omitempty" json:"ip"`
}

type LxdNodeOpts struct {
	Image       string                 `yaml:"image,omitempty" json:"image"`
	Server      string                 `yaml:"server,omitempty" json:"server"`
	Protocol    string                 `yaml:"protocol,omitempty" json:"protocol"`
	ExecOptions map[string]interface{} `yaml:"exec_opts,omitempty" json:"exec_opts"`
	Networks    []LxdNetworkOpts       `yaml:"networks,omitempty" json:"networks"`
}

func NewLxdNode(name string, opts NodeOptions) (node Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts LxdNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	// Set defaults
	if nodeopts.Server == "" {
		nodeopts.Server = "local"
	}
	if nodeopts.Protocol == "" {
		nodeopts.Protocol = "lxd"
	}

	// If image contains a name:alias, split it and configure the server and protocol
	if strings.Contains(nodeopts.Image, ":") {
		server, protocol, err := lxc.GetUrlAndProtocol(strings.Split(nodeopts.Image, ":")[0])
		if err != nil {
			return nil, err
		}
		nodeopts.Image = strings.Split(nodeopts.Image, ":")[1]
		nodeopts.Server = server
		nodeopts.Protocol = protocol
	}

	client, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		return nil, err
	}

	return &LxdNode{
		name:    name,
		options: nodeopts,
		client:  client,
	}, nil

}

type LxdNode struct {
	name      string
	client    lxd.InstanceServer
	options   LxdNodeOpts
	addresses []string
}

func (d *LxdNode) Setup() error {
	if d.client == nil {
		return helpers.WrapError("lxd client not initialized")
	}

	// Create a request for the container
	req := api.InstancesPost{
		Name: d.name,
		Source: api.InstanceSource{
			Type:     "image",
			Alias:    d.options.Image,
			Server:   d.options.Server,
			Protocol: d.options.Protocol,
		},
		Type: "container",
	}

	op, err := d.client.CreateInstance(req)
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error creating container: %w", err))
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error creating container: %w", err))
	}

	// Start the container
	reqState := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err = d.client.UpdateInstanceState(d.name, reqState, "")
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error starting container: %w", err))
	}

	return op.Wait()
}

func (d *LxdNode) Teardown() error {
	if d.client == nil {
		return helpers.WrapError("lxd client not initialized")
	}

	// Create a stop request
	req := api.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
		Force:   true,
	}
	op, err := d.client.UpdateInstanceState(d.name, req, "")
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error stopping container: %w", err))
	}
	if err = op.Wait(); err != nil {
		return helpers.WrapError(fmt.Sprintf("error stopping container: %w", err))
	}

	// Create a delete request
	op, err = d.client.DeleteInstance(d.name)
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error deleting container: %w", err))
	}
	if err = op.Wait(); err != nil {
		return helpers.WrapError(fmt.Sprintf("error deleting container: %w", err))
	}

	return nil
}

func (d *LxdNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {

	if d.client == nil {
		return nil, helpers.WrapError("lxd client not initialized")
	}

	var wout, werr bytes.Buffer
	execArgs := lxd.InstanceExecArgs{
		Stdout: &wout,
		Stderr: &werr,
	}

	//// Split the command into a slice naively just split on spaces
	//cmdparts, err := Fields(command)
	//if err != nil {
	//	return nil, helpers.WrapError(fmt.Sprintf("error splitting command: %w", err))
	//}
	//
	//// TODO: I need to figure out how to handle piped command like I do in the command executor
	//// TODO: Check d.options.ExecOptions for a shell and if there is one make sure we execute commands inside of it

	// Find the full path to the binary
	execLookPath := api.InstanceExecPost{
		Command:     []string{"/bin/bash", "-c", command},
		WaitForWS:   true,
		Interactive: false,
	}

	op, err := d.client.ExecInstance(d.name, execLookPath, &execArgs)
	if err != nil {
		return nil, helpers.WrapError(fmt.Sprintf("error executing command: %s", err.Error()))
	}

	if err = op.Wait(); err != nil {
		return nil, helpers.WrapError(fmt.Sprintf("error executing command: %s", err.Error()))
	}

	//execPath := strings.TrimSpace(wout.String())
	//wout.Reset()
	//werr.Reset()
	//
	//execArgs = lxd.InstanceExecArgs{
	//	Stdout: &wout,
	//	Stderr: &werr,
	//}
	//
	//cmd := append([]string{execPath}, cmdparts[1:]...)
	//execPost := api.InstanceExecPost{
	//	Command:     cmd,
	//	WaitForWS:   true,
	//	Interactive: false,
	//}
	//
	//op, err = d.client.ExecInstance(d.name, execPost, &execArgs)
	//if err != nil {
	//	return nil, helpers.WrapError(fmt.Sprintf("error executing command: %w", err))
	//}
	//
	//if err = op.Wait(); err != nil {
	//	return nil, helpers.WrapError(fmt.Sprintf("error executing command: %w", err))
	//}

	metadata := op.Get().Metadata
	exitCode, ok := metadata["return"].(float64)
	if !ok {
		return nil, helpers.WrapError("error getting exit code")
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    int(exitCode),
		Stdout:      &wout,
		Stderr:      &werr,
	}, nil
}

func (d *LxdNode) Close() error {
	//TODO implement me
	return helpers.WrapError("not implemented")
}

func Fields(s string) ([]string, error) {
	var (
		fields        []string
		field         strings.Builder
		inSingleQuote bool
		inDoubleQuote bool
		escaping      bool
	)

	for _, r := range s {
		if escaping {
			// If the previous character was a backslash, just add this rune.
			field.WriteRune(r)
			escaping = false
			continue
		}
		switch r {
		case '\\':
			escaping = true
		case '\'':
			if !inDoubleQuote {
				// Toggle single quote state but do not include the quote in the output.
				inSingleQuote = !inSingleQuote
				continue
			}
			// If inside a double quote, treat it as a normal character.
			field.WriteRune(r)
		case '"':
			if !inSingleQuote {
				// Toggle double quote state but do not include the quote.
				inDoubleQuote = !inDoubleQuote
				continue
			}
			// If inside a single quote, treat it as a normal character.
			field.WriteRune(r)
		default:
			if unicode.IsSpace(r) && !inSingleQuote && !inDoubleQuote {
				if field.Len() > 0 {
					fields = append(fields, field.String())
					field.Reset()
				}
			} else {
				field.WriteRune(r)
			}
		}
	}

	// If an escape character was left dangling, add it literally.
	if escaping {
		field.WriteRune('\\')
	}

	// Append the final field if non-empty.
	if field.Len() > 0 {
		fields = append(fields, field.String())
	}

	if inSingleQuote || inDoubleQuote {
		return nil, errors.New("unclosed quote")
	}

	return fields, nil
}
