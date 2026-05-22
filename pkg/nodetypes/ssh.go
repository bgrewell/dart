package nodetypes

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/pkg/sftp"
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
		config: config,
		client: client,
	}, nil
}

type SshNode struct {
	config  *ssh.ClientConfig
	client  *ssh.Client
	address string

	sftpMu sync.Mutex
	sftp   *sftp.Client
}

func (s *SshNode) Setup() error {
	return nil
}

func (s *SshNode) Teardown() error {
	return nil
}

func (s *SshNode) Close() error {

	s.sftpMu.Lock()
	if s.sftp != nil {
		s.sftp.Close()
		s.sftp = nil
	}
	s.sftpMu.Unlock()

	// Close the client
	if s.client != nil {
		s.client.Close()
	}

	return nil
}

// getSftp lazily opens and caches an SFTP session over the existing SSH client.
func (s *SshNode) getSftp() (*sftp.Client, error) {
	s.sftpMu.Lock()
	defer s.sftpMu.Unlock()
	if s.sftp != nil {
		return s.sftp, nil
	}
	c, err := sftp.NewClient(s.client)
	if err != nil {
		return nil, fmt.Errorf("failed to open sftp session: %w", err)
	}
	s.sftp = c
	return c, nil
}

func (s *SshNode) ReadFile(path string) ([]byte, error) {
	c, err := s.getSftp()
	if err != nil {
		return nil, err
	}
	f, err := c.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *SshNode) WriteFile(path string, data []byte, mode fs.FileMode) error {
	c, err := s.getSftp()
	if err != nil {
		return err
	}
	f, err := c.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

func (s *SshNode) RemoveFile(path string) error {
	c, err := s.getSftp()
	if err != nil {
		return err
	}
	return c.Remove(path)
}

func (s *SshNode) Stat(path string) (ifaces.FileInfo, error) {
	c, err := s.getSftp()
	if err != nil {
		return ifaces.FileInfo{}, err
	}
	info, err := c.Stat(path)
	if err != nil {
		return ifaces.FileInfo{}, err
	}
	return ifaces.FileInfo{Size: info.Size(), Mode: info.Mode(), IsDir: info.IsDir()}, nil
}

func (s *SshNode) MkdirAll(path string, mode fs.FileMode) error {
	c, err := s.getSftp()
	if err != nil {
		return err
	}
	if err := c.MkdirAll(path); err != nil {
		return err
	}
	// MkdirAll in pkg/sftp does not let us pass a mode; chmod the leaf to honor the requested mode.
	return c.Chmod(path, mode)
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
