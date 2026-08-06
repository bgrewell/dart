package nodetypes

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"golang.org/x/crypto/ssh"
)

var _ ifaces.Node = &SshNode{}

type SshNodeOpts struct {
	Host    string `yaml:"host,omitempty" json:"host"`
	Port    int    `yaml:"port,omitempty" json:"port"`
	User    string `yaml:"user,omitempty" json:"user"`
	Pass    string `yaml:"pass,omitempty" json:"pass"`
	KeyFile string `yaml:"key,omitempty" json:"key"`
}

func NewSshNode(opts ifaces.NodeOptions) (node ifaces.Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts SshNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	if nodeopts.Port == 0 {
		nodeopts.Port = 22
	}

	addr := fmt.Sprintf("%s:%d", nodeopts.Host, nodeopts.Port)

	authMethods := []ssh.AuthMethod{}
	if nodeopts.KeyFile != "" {
		signer, err := readPrivateKey(nodeopts.KeyFile)
		if err != nil {
			return nil, err
		}
		auth := ssh.PublicKeys(signer)
		authMethods = append(authMethods, auth)
	}
	if nodeopts.Pass != "" {
		authMethods = append(authMethods, ssh.Password(nodeopts.Pass))
	}

	// Create a new ssh configuration
	config := &ssh.ClientConfig{
		Config:          ssh.Config{},
		User:            nodeopts.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Dial the ssh server
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	// Return the new ssh node
	return &SshNode{
		config:  config,
		client:  client,
		address: addr,
	}, nil
}

var _ ifaces.Rebooter = &SshNode{}

// Reboot issues a reboot on the remote host and reconnects until it accepts
// commands again. The reboot command tries passwordless sudo first and
// falls back to a direct reboot for root sessions; force adds -f (no clean
// shutdown). A zero timeout waits up to five minutes.
func (s *SshNode) Reboot(force bool, readyCommand string, timeout time.Duration) error {
	command := "sudo -n reboot || reboot"
	if force {
		command = "sudo -n reboot -f || reboot -f"
	}
	// Best-effort: the connection usually drops before the command returns
	_, _ = s.Execute(command)
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if readyCommand == "" {
		readyCommand = "true"
	}

	// Give the host a moment to actually go down so the poll below can't
	// succeed against the pre-reboot system
	time.Sleep(5 * time.Second)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.client == nil {
			client, err := ssh.Dial("tcp", s.address, s.config)
			if err != nil {
				time.Sleep(3 * time.Second)
				continue
			}
			s.client = client
		}
		result, err := s.Execute(readyCommand)
		if err == nil && result.ExitCode == 0 {
			return nil
		}
		if err != nil {
			// Connection died again (host still shutting down); redial
			s.client.Close()
			s.client = nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timeout waiting for %s to accept commands after reboot", s.address)
}

type SshNode struct {
	config  *ssh.ClientConfig
	client  *ssh.Client
	address string
}

func (s *SshNode) Setup() error {
	return nil
}

func (s *SshNode) Teardown() error {
	return nil
}

func (s *SshNode) Close() error {

	// Close the client
	if s.client != nil {
		s.client.Close()
	}

	return nil
}

// Execute runs a command on the remote SSH host.
//
// TODO: Implement debug streaming output for SSH node.
//
// Current Issue: The SSH implementation has a bug where session.StdoutPipe() and
// session.StderrPipe() return pipes that are consumed by session.Run() before
// the Execute() method returns. By the time the caller tries to read from the
// returned readers, the pipes are already closed/empty.
//
// Solution: Replace the pipe-based approach with direct writers:
//  1. Create TeeWriter instances for stdout and stderr
//  2. Assign them to session.Stdout and session.Stderr (instead of using pipes)
//  3. Call session.Run() which writes directly to our TeeWriters
//  4. Return TeeWriter.Reader() for both streams
//
// This fix also resolves the existing bug where output is lost even without debug mode.
func (s *SshNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {

	// Create a new session
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}

	// Set up the pipes to capture stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, err
	}

	// Run the command
	exitCode := 0
	err = session.Run(command)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("failed to get exit code: %v", err)
		}
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    exitCode,
		Stdout:      stdout,
		Stderr:      stderr,
	}, nil
}

// readPrivateKey reads an SSH private key from a file
func readPrivateKey(file string) (ssh.Signer, error) {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}

	return key, nil
}
