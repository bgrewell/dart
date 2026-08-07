package nodetypes

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// startTestSSHServer runs a minimal in-process SSH server that accepts
// password auth and answers exec requests with canned responses:
//   - "echo hello"  -> stdout "hello\n", exit 0
//   - "fail"        -> stderr "boom\n", exit 3
//   - anything else -> stdout echoes the command, exit 0
func startTestSSHServer(t *testing.T) (host string, port int) {
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
	return "127.0.0.1", addr.Port
}

func handleTestSSHConn(conn net.Conn, config *ssh.ServerConfig) {
	serverConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
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
		"host": host,
		"port": port,
		"user": "testuser",
		"pass": "testpass",
	}
	node, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts))
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
		"host": host,
		"port": port,
		"user": "testuser",
		"pass": "wrong",
	}
	_, err := NewSshNode("ssh-test", ifaces.NodeOptions(&opts))
	assert.Error(t, err)
}
