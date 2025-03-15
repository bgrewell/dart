package nodetypes

import (
	"encoding/json"
	"fmt"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"golang.org/x/crypto/ssh"
	"os"
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
		config: config,
		client: client,
	}, nil
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
