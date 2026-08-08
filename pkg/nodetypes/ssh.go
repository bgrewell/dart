package nodetypes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dartconfig "github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/bgrewell/dart/pkg/ifaces"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var _ ifaces.Node = &SshNode{}

type SshNodeOpts struct {
	Host    string `yaml:"host,omitempty" json:"host"`
	Port    int    `yaml:"port,omitempty" json:"port"`
	User    string `yaml:"user,omitempty" json:"user"`
	Pass    string `yaml:"pass,omitempty" json:"pass"`
	KeyFile string `yaml:"key,omitempty" json:"key"`
	// KnownHosts verifies the server against an OpenSSH known_hosts file.
	// InsecureSkipHostKey opts out explicitly; without either, DART looks
	// for ~/.ssh/known_hosts and refuses to connect if it cannot verify —
	// silently trusting any key is not a safe default for a tool that
	// carries credentials.
	KnownHosts          string `yaml:"known_hosts,omitempty" json:"known_hosts"`
	InsecureSkipHostKey bool   `yaml:"insecure_skip_host_key,omitempty" json:"insecure_skip_host_key"`
	// Bastion routes the connection through a jump host, for lab networks
	// where targets are not directly reachable.
	Bastion *SshBastionOpts `yaml:"bastion,omitempty" json:"bastion"`
}

// SshBastionOpts describes the jump host used to reach a node.
type SshBastionOpts struct {
	Host    string `yaml:"host,omitempty" json:"host"`
	Port    int    `yaml:"port,omitempty" json:"port"`
	User    string `yaml:"user,omitempty" json:"user"`
	Pass    string `yaml:"pass,omitempty" json:"pass"`
	KeyFile string `yaml:"key,omitempty" json:"key"`
	// The bastion may set its own host-key policy; without one it inherits
	// the target's. A jump host usually outlives the targets behind it, so
	// relaxing verification for an ephemeral target should not silently
	// relax it for the bastion.
	KnownHosts          string `yaml:"known_hosts,omitempty" json:"known_hosts"`
	InsecureSkipHostKey *bool  `yaml:"insecure_skip_host_key,omitempty" json:"insecure_skip_host_key"`
	// Bastion is rejected rather than ignored: chained jump hosts are not
	// supported, and silently dropping the inner hop would connect
	// somewhere the suite did not intend.
	Bastion *SshBastionOpts `yaml:"bastion,omitempty" json:"bastion"`
}

func NewSshNode(name string, opts ifaces.NodeOptions, suiteDir string) (node ifaces.Node, err error) {

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

	// Credential paths belong to the machine running DART, so they follow
	// the suite-relative rule like every other local path
	if nodeopts.KeyFile, err = dartconfig.ResolveLocalPath(suiteDir, nodeopts.KeyFile); err != nil {
		return nil, err
	}
	if nodeopts.KnownHosts, err = dartconfig.ResolveLocalPath(suiteDir, nodeopts.KnownHosts); err != nil {
		return nil, err
	}
	if nodeopts.Bastion != nil {
		if nodeopts.Bastion.KeyFile, err = dartconfig.ResolveLocalPath(suiteDir, nodeopts.Bastion.KeyFile); err != nil {
			return nil, err
		}
		if nodeopts.Bastion.KnownHosts, err = dartconfig.ResolveLocalPath(suiteDir, nodeopts.Bastion.KnownHosts); err != nil {
			return nil, err
		}
	}

	authMethods, err := sshAuthMethods(nodeopts.KeyFile, nodeopts.Pass)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := hostKeyCallbackFor(nodeopts.KnownHosts, nodeopts.InsecureSkipHostKey)
	if err != nil {
		return nil, err
	}

	// Create a new ssh configuration
	config := &ssh.ClientConfig{
		Config:          ssh.Config{},
		User:            nodeopts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	// Dial the target, optionally through a bastion
	var client *ssh.Client
	var bastionClient *ssh.Client
	if nodeopts.Bastion != nil {
		bastionClient, err = dialBastion(nodeopts.Bastion, nodeopts.KnownHosts, nodeopts.InsecureSkipHostKey)
		if err != nil {
			return nil, err
		}
		conn, err := bastionClient.Dial("tcp", addr)
		if err != nil {
			bastionClient.Close()
			return nil, fmt.Errorf("bastion could not reach %s: %w", addr, err)
		}
		ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
		if err != nil {
			bastionClient.Close()
			return nil, fmt.Errorf("ssh handshake with %s via bastion failed: %w", addr, err)
		}
		client = ssh.NewClient(ncc, chans, reqs)
	} else {
		client, err = ssh.Dial("tcp", addr, config)
		if err != nil {
			return nil, describeSSHDialError(name, addr, err)
		}
	}

	// Return the new ssh node
	return &SshNode{
		name:                name,
		config:              config,
		client:              client,
		address:             addr,
		bastion:             bastionClient,
		bastions:            nodeopts.Bastion,
		knownHosts:          nodeopts.KnownHosts,
		insecureSkipHostKey: nodeopts.InsecureSkipHostKey,
	}, nil
}

// describeSSHDialError names the node and address, and points at the
// host-key options when verification is what failed — the bare library
// error ("knownhosts: key mismatch") says neither which node nor what to
// do about it.
func describeSSHDialError(nodeName, addr string, err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "knownhosts: key is unknown"):
		return fmt.Errorf("node %q: host key for %s is not in known_hosts: %w (add it, set known_hosts: <path>, or set insecure_skip_host_key: true)", nodeName, addr, err)
	case strings.Contains(message, "knownhosts: key mismatch"):
		return fmt.Errorf("node %q: host key for %s does not match known_hosts: %w (the host was rebuilt or the connection is being intercepted; update known_hosts deliberately)", nodeName, addr, err)
	default:
		return fmt.Errorf("node %q: cannot connect to %s: %w", nodeName, addr, err)
	}
}

// sshAuthMethods builds the auth chain from a key file and/or password.
func sshAuthMethods(keyFile, password string) ([]ssh.AuthMethod, error) {
	methods := []ssh.AuthMethod{}
	if keyFile != "" {
		signer, err := readPrivateKey(expandUser(keyFile))
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if password != "" {
		methods = append(methods, ssh.Password(password))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no ssh credentials configured: set key or pass")
	}
	return methods, nil
}

// hostKeyCallbackFor resolves host-key verification. Verification is the
// default: an unverifiable host is an error unless the suite explicitly
// opts out, so a tool holding credentials cannot silently trust any key.
func hostKeyCallbackFor(knownHosts string, insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	path := expandUser(knownHosts)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot locate known_hosts: %w (set known_hosts or insecure_skip_host_key)", err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("known_hosts file %s is unreadable: %w (set known_hosts or insecure_skip_host_key: true)", path, err)
	}

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("cannot use known_hosts %s: %w", path, err)
	}
	return callback, nil
}

// dialBastion connects to the jump host, reusing the node's host-key
// policy so the bastion is verified on the same terms as the target.
func dialBastion(opts *SshBastionOpts, knownHosts string, insecure bool) (*ssh.Client, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("bastion host is required")
	}
	if opts.Bastion != nil {
		return nil, fmt.Errorf("chained bastions are not supported: remove the nested bastion block")
	}
	port := opts.Port
	if port == 0 {
		port = 22
	}

	// The bastion's own policy wins when set; otherwise it inherits the
	// target's
	bastionKnownHosts := knownHosts
	if opts.KnownHosts != "" {
		bastionKnownHosts = opts.KnownHosts
	}
	bastionInsecure := insecure
	if opts.InsecureSkipHostKey != nil {
		bastionInsecure = *opts.InsecureSkipHostKey
	}

	authMethods, err := sshAuthMethods(opts.KeyFile, opts.Pass)
	if err != nil {
		return nil, fmt.Errorf("bastion: %w", err)
	}
	callback, err := hostKeyCallbackFor(bastionKnownHosts, bastionInsecure)
	if err != nil {
		return nil, fmt.Errorf("bastion: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", opts.Host, port)
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: callback,
	})
	if err != nil {
		return nil, describeSSHDialError("bastion "+opts.Host, addr, err)
	}
	return client, nil
}

// expandUser resolves a leading ~ in a path.
func expandUser(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
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
	// Best-effort: the connection usually drops before the command returns.
	// A previous failed reboot can leave the client closed, so reconnect
	// first rather than dereferencing nil.
	if s.client == nil {
		client, dialErr := s.redial()
		if dialErr != nil {
			return fmt.Errorf("cannot reach %s to reboot it: %w", s.address, dialErr)
		}
		s.client = client
	}
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
			client, err := s.redial()
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
	name                string
	config              *ssh.ClientConfig
	client              *ssh.Client
	address             string
	bastion             *ssh.Client
	bastions            *SshBastionOpts
	knownHosts          string
	insecureSkipHostKey bool
}

func (s *SshNode) Setup() error {
	return nil
}

func (s *SshNode) Teardown() error {
	return nil
}

func (s *SshNode) Close() error {
	if s.client != nil {
		s.client.Close()
	}
	// The bastion tunnel outlives the target connection and must be
	// released too, or the jump host accumulates sessions across runs
	if s.bastion != nil {
		s.bastion.Close()
	}
	return nil
}

// Execute runs a command on the remote SSH host. Output is captured into
// tee writers assigned as the session's Stdout/Stderr — session pipes are
// unusable here because Run drains and closes them before returning, which
// previously lost all command output. The tee writers also provide debug
// streaming, matching the other node types.
func (s *SshNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	if s.client == nil {
		return nil, fmt.Errorf("ssh node %s has no open connection to %s", s.name, s.address)
	}

	// Create a new session
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	debugEnabled := execution.IsDebugMode()
	stdoutWriter := stream.NewTeeWriter(stream.StreamStdout, s.name, debugEnabled)
	stderrWriter := stream.NewTeeWriter(stream.StreamStderr, s.name, debugEnabled)
	session.Stdout = stdoutWriter
	session.Stderr = stderrWriter

	// Run the command
	exitCode := 0
	err = session.Run(command)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("ssh command failed: %w", err)
		}
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    exitCode,
		Stdout:      stdoutWriter.Reader(),
		Stderr:      stderrWriter.Reader(),
	}, nil
}

// redial reconnects to the target, routing through the bastion when one
// is configured — a rebooted host behind a jump host must be reached the
// same way it was originally.
func (s *SshNode) redial() (*ssh.Client, error) {
	if s.bastions == nil {
		return ssh.Dial("tcp", s.address, s.config)
	}

	if s.bastion != nil {
		s.bastion.Close()
	}
	bastionClient, err := dialBastion(s.bastions, s.knownHosts, s.insecureSkipHostKey)
	if err != nil {
		return nil, err
	}

	// Only adopt the bastion once the target is reachable through it;
	// otherwise a failed hop would leave an authenticated session open on
	// the jump host for the rest of the run
	conn, err := bastionClient.Dial("tcp", s.address)
	if err != nil {
		bastionClient.Close()
		return nil, err
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, s.address, s.config)
	if err != nil {
		bastionClient.Close()
		return nil, err
	}
	s.bastion = bastionClient
	return ssh.NewClient(ncc, chans, reqs), nil
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
