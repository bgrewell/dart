package nodetypes

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// startTestSSHServer runs a minimal in-process SSH server that accepts
// password auth and answers exec requests with canned responses:
//   - "echo hello"  -> stdout "hello\n", exit 0
//   - "fail"        -> stderr "boom\n", exit 3
//   - anything else -> stdout echoes the command, exit 0
func startTestSSHServer(t *testing.T) (host string, port int) {
	h, p, _ := startTestSSHServerWithKey(t)
	return h, p
}

func startTestSSHServerWithKey(t *testing.T) (host string, port int, publicKey ssh.PublicKey) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if conn.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, io.EOF
		},
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestSSHConn(conn, serverConfig)
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, signer.PublicKey()
}

func handleTestSSHConn(conn net.Conn, config *ssh.ServerConfig) {
	serverConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		// direct-tcpip makes the test server usable as a jump host: it
		// dials the requested address and proxies bytes, exactly as a
		// real bastion does
		if newChannel.ChannelType() == "direct-tcpip" {
			var payload struct {
				DestHost   string
				DestPort   uint32
				OriginHost string
				OriginPort uint32
			}
			if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
				newChannel.Reject(ssh.ConnectionFailed, "bad payload")
				continue
			}
			target, err := net.Dial("tcp", net.JoinHostPort(payload.DestHost, strconv.Itoa(int(payload.DestPort))))
			if err != nil {
				newChannel.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				target.Close()
				continue
			}
			go ssh.DiscardRequests(requests)
			go func() {
				defer channel.Close()
				defer target.Close()
				go io.Copy(target, channel)
				io.Copy(channel, target)
			}()
			continue
		}
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func(channel ssh.Channel, requests <-chan *ssh.Request) {
			defer channel.Close()
			for req := range requests {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				ssh.Unmarshal(req.Payload, &payload)
				req.Reply(true, nil)

				status := uint32(0)
				switch payload.Command {
				case "echo hello":
					io.WriteString(channel, "hello\n")
				case "fail":
					io.WriteString(channel.Stderr(), "boom\n")
					status = 3
				default:
					io.WriteString(channel, payload.Command+"\n")
				}

				channel.SendRequest("exit-status", false,
					ssh.Marshal(struct{ Status uint32 }{status}))
				return
			}
		}(channel, requests)
	}
}

func testSSHNode(t *testing.T) ifaces.Node {
	t.Helper()
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host":                   host,
		"port":                   port,
		"user":                   "testuser",
		"pass":                   "testpass",
		"insecure_skip_host_key": true,
	}
	node, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.NoError(t, err)
	t.Cleanup(func() { node.Close() })
	return node
}

// The output-capture regression: session pipes are drained before Execute
// returns, so output must be captured via writers. Stdout must arrive
// intact.
func TestSSHExecuteCapturesStdout(t *testing.T) {
	node := testSSHNode(t)

	result, err := node.Execute("echo hello")
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)

	stdout, err := result.StdoutBytes()
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(stdout))
}

func TestSSHExecuteCapturesStderrAndExitCode(t *testing.T) {
	node := testSSHNode(t)

	result, err := node.Execute("fail")
	require.NoError(t, err)
	assert.Equal(t, 3, result.ExitCode)

	stderr, err := result.StderrBytes()
	require.NoError(t, err)
	assert.Equal(t, "boom\n", string(stderr))
}

// Sequential commands each get a fresh session; output does not bleed
// between executions.
func TestSSHExecuteSequentialCommands(t *testing.T) {
	node := testSSHNode(t)

	for i := 0; i < 3; i++ {
		command := "run-" + strconv.Itoa(i)
		result, err := node.Execute(command)
		require.NoError(t, err)
		stdout, err := result.StdoutBytes()
		require.NoError(t, err)
		assert.Equal(t, command+"\n", strings.TrimRight(string(stdout), "")) // includes trailing newline
	}
}

func TestSSHBadCredentials(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host":                   host,
		"port":                   port,
		"user":                   "testuser",
		"pass":                   "wrong",
		"insecure_skip_host_key": true,
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	assert.Error(t, err)
}

// Host keys are verified by default: an unknown server is refused rather
// than silently trusted.
func TestSSHUnknownHostKeyRefused(t *testing.T) {
	host, port := startTestSSHServer(t)
	emptyKnownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(emptyKnownHosts, []byte{}, 0600))

	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"known_hosts": emptyKnownHosts,
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knownhosts")
}

// A host listed in known_hosts connects normally.
func TestSSHKnownHostAccepted(t *testing.T) {
	host, port, hostKey := startTestSSHServerWithKey(t)
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	entry := knownhosts.Line([]string{fmt.Sprintf("[%s]:%d", host, port)}, hostKey)
	require.NoError(t, os.WriteFile(knownHostsPath, []byte(entry+"\n"), 0600))

	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"known_hosts": knownHostsPath,
	}
	node, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.NoError(t, err)
	defer node.Close()

	result, err := node.Execute("echo hello")
	require.NoError(t, err)
	stdout, _ := result.StdoutBytes()
	assert.Equal(t, "hello\n", string(stdout))
}

// A missing known_hosts file is a clear error naming the way out.
func TestSSHMissingKnownHostsExplains(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"known_hosts": filepath.Join(t.TempDir(), "absent"),
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insecure_skip_host_key")
}

func TestSSHNoCredentials(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser",
		"insecure_skip_host_key": true,
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ssh credentials")
}

// Connecting through a bastion reaches a target that is only routable
// from the jump host's perspective.
func TestSSHThroughBastion(t *testing.T) {
	targetHost, targetPort := startTestSSHServer(t)
	bastionHost, bastionPort := startTestSSHServer(t)

	opts := map[string]interface{}{
		"host": targetHost, "port": targetPort,
		"user": "testuser", "pass": "testpass",
		"insecure_skip_host_key": true,
		"bastion": map[string]interface{}{
			"host": bastionHost, "port": bastionPort,
			"user": "testuser", "pass": "testpass",
		},
	}
	node, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.NoError(t, err)
	defer node.Close()

	result, err := node.Execute("echo hello")
	require.NoError(t, err)
	stdout, _ := result.StdoutBytes()
	assert.Equal(t, "hello\n", string(stdout))
}

func TestSSHBastionValidation(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"insecure_skip_host_key": true,
		"bastion":                map[string]interface{}{"user": "testuser", "pass": "x"},
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bastion host is required")
}

// A failed reboot leaves the client closed; a later reboot must reconnect
// rather than dereference nil and crash the run.
func TestSSHRebootAfterClosedClientDoesNotPanic(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"insecure_skip_host_key": true,
	}
	node, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.NoError(t, err)

	sshNode := node.(*SshNode)
	sshNode.client.Close()
	sshNode.client = nil

	// Executing with no connection is an error, never a panic
	_, err = sshNode.Execute("echo hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open connection")

	// Reboot reconnects first; the test server has no reboot semantics, so
	// this returns a timeout error — the point is that it does not panic
	assert.NotPanics(t, func() {
		_ = sshNode.Reboot(false, "true", 1)
	})
}

func TestSSHHostKeyErrorsNameTheNodeAndOptions(t *testing.T) {
	host, port, hostKey := startTestSSHServerWithKey(t)
	_ = hostKey

	emptyKnownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(emptyKnownHosts, []byte{}, 0600))
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"known_hosts": emptyKnownHosts,
	}
	_, err := NewSshNode("prod-web", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod-web", "the failing node must be named")
	assert.Contains(t, err.Error(), "insecure_skip_host_key", "the way out must be named")
}

// A key that is present but different is the interception case and must
// say so.
func TestSSHHostKeyMismatchExplains(t *testing.T) {
	host, port, _ := startTestSSHServerWithKey(t)
	_, _, otherKey := startTestSSHServerWithKey(t)

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	entry := knownhosts.Line([]string{fmt.Sprintf("[%s]:%d", host, port)}, otherKey)
	require.NoError(t, os.WriteFile(knownHostsPath, []byte(entry+"\n"), 0600))

	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"known_hosts": knownHostsPath,
	}
	_, err := NewSshNode("prod-web", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match known_hosts")
	assert.Contains(t, err.Error(), "intercepted")
}

func TestSSHChainedBastionRejected(t *testing.T) {
	host, port := startTestSSHServer(t)
	opts := map[string]interface{}{
		"host": host, "port": port, "user": "testuser", "pass": "testpass",
		"insecure_skip_host_key": true,
		"bastion": map[string]interface{}{
			"host": "jump1", "user": "u", "pass": "p",
			"bastion": map[string]interface{}{"host": "jump2", "user": "u", "pass": "p"},
		},
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chained bastions are not supported")
}

// The bastion can keep verification on while the ephemeral target opts out.
func TestSSHBastionKeepsOwnHostKeyPolicy(t *testing.T) {
	targetHost, targetPort := startTestSSHServer(t)
	bastionHost, bastionPort := startTestSSHServer(t)
	emptyKnownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(emptyKnownHosts, []byte{}, 0600))

	insecure := false
	opts := map[string]interface{}{
		"host": targetHost, "port": targetPort, "user": "testuser", "pass": "testpass",
		"insecure_skip_host_key": true,
		"bastion": map[string]interface{}{
			"host": bastionHost, "port": bastionPort, "user": "testuser", "pass": "testpass",
			"known_hosts": emptyKnownHosts, "insecure_skip_host_key": insecure,
		},
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts), "")
	require.Error(t, err, "the bastion must still verify even though the target does not")
	assert.Contains(t, err.Error(), "bastion")
}
