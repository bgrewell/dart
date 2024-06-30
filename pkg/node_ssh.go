package pkg

import (
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"golang.org/x/crypto/ssh"
)

var _ Node = &SshNode{}

func NewSshNode(opts NodeOptions) (node Node, err error) {
	// Create a new ssh configuration
	config := &ssh.ClientConfig{
		Config: ssh.Config{},
		User:   "username",
		Auth: []ssh.AuthMethod{
			ssh.Password(""),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Dial the ssh server
	client, err := ssh.Dial("tcp", "hostname:22", config)
	if err != nil {
		return nil, err
	}

	// Create a new session TODO: Probably should just be done each execution
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	// Return the new ssh node
	return &SshNode{
		config:  config,
		client:  client,
		session: session,
	}, nil
}

type SshNode struct {
	config  *ssh.ClientConfig
	client  *ssh.Client
	session *ssh.Session
}

func (s *SshNode) Setup() error {
	//TODO implement me
	return helpers.WrapError("not implemented")
}

func (s *SshNode) Teardown() error {
	//TODO implement me
	return helpers.WrapError("not implemented")
}

func (s *SshNode) Close() error {

	// Close the session
	if s.session != nil {
		s.session.Close()
	}

	// Close the client
	if s.client != nil {
		s.client.Close()
	}

	return nil
}

func (s *SshNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	return nil, helpers.WrapError("not implemented")
}
