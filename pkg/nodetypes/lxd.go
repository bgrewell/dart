package nodetypes

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/internal/lxc"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/internal/platform"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/bgrewell/dart/pkg/ifaces"
	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

var _ ifaces.Node = &LxdNode{}

type LxdNetworkOpts struct {
	Name   string `yaml:"name,omitempty" json:"name"`
	Subnet string `yaml:"subnet,omitempty" json:"subnet"`
	Ip     string `yaml:"ip,omitempty" json:"ip"`
}

// LxdBootWaitOpts controls how long to wait for an instance to start accepting
// commands after it has been started. It is intended for instances that boot from an
// installer ISO, where the instance is unreachable for the duration of the install and
// only becomes usable once it has rebooted from disk.
type LxdBootWaitOpts struct {
	Timeout      int    `yaml:"timeout,omitempty" json:"timeout"`             // Maximum seconds to wait for the instance to become ready
	Interval     int    `yaml:"interval,omitempty" json:"interval"`           // Seconds between readiness checks
	InitialDelay int    `yaml:"initial_delay,omitempty" json:"initial_delay"` // Seconds to wait before the first readiness check
	ReadyCommand string `yaml:"ready_command,omitempty" json:"ready_command"` // Command that must exit zero for the instance to be considered ready
}

type LxdNodeOpts struct {
	Image        string                            `yaml:"image,omitempty" json:"image"`
	Empty        bool                              `yaml:"empty,omitempty" json:"empty"` // Create an instance with no image (boot media is supplied via devices)
	Server       string                            `yaml:"server,omitempty" json:"server"`
	Protocol     string                            `yaml:"protocol,omitempty" json:"protocol"`
	InstanceType string                            `yaml:"instance_type,omitempty" json:"instance_type"` // "container" or "virtual-machine"
	Profiles     []string                          `yaml:"profiles,omitempty" json:"profiles"`
	Config       map[string]interface{}            `yaml:"config,omitempty" json:"config"`   // Instance configuration keys (e.g. security.secureboot)
	Devices      map[string]map[string]interface{} `yaml:"devices,omitempty" json:"devices"` // Arbitrary instance devices (ISO disks, extra disks, ...)
	BootWait     *LxdBootWaitOpts                  `yaml:"boot_wait,omitempty" json:"boot_wait"`
	ExecOptions  map[string]interface{}            `yaml:"exec_opts,omitempty" json:"exec_opts"`
	Networks     []LxdNetworkOpts                  `yaml:"networks,omitempty" json:"networks"`
	// Socket path for local connections (supports both LXD and Incus)
	Socket string `yaml:"socket,omitempty" json:"socket"`
	// Remote connection options (for connecting to remote LXD servers)
	RemoteAddr string `yaml:"remote_addr,omitempty" json:"remote_addr"` // HTTPS address for remote LXD server (e.g., "https://10.0.0.1:8443")
	ClientCert string `yaml:"client_cert,omitempty" json:"client_cert"` // Path to client certificate file
	ClientKey  string `yaml:"client_key,omitempty" json:"client_key"`   // Path to client key file
	ServerCert string `yaml:"server_cert,omitempty" json:"server_cert"` // Path to server certificate file (optional, for custom CA)
	TrustToken string `yaml:"trust_token,omitempty" json:"trust_token"` // One-time trust token from 'lxc config trust add' (modern authentication)
	SkipVerify bool   `yaml:"skip_verify,omitempty" json:"skip_verify"` // Skip TLS verification (not recommended for production)
	// Project support
	Project string `yaml:"project,omitempty" json:"project"` // LXD project to use (defaults to lxd.DefaultProject)
}

// emptyInstance reports whether the instance should be created without an image.
// An instance is empty when it is explicitly marked as such or when no image was given.
func (o LxdNodeOpts) emptyInstance() bool {
	return o.Empty || o.Image == ""
}

// validate checks the option combinations that cannot be caught by the LXD server
// until well after the instance has been created.
func (o LxdNodeOpts) validate() error {
	if o.Empty && o.Image != "" {
		return helpers.WrapError("empty instances cannot specify an image; remove either 'empty' or 'image'")
	}
	return nil
}

// readinessConfig converts the boot wait options into a readiness configuration,
// substituting defaults for any value that was not provided.
func (b *LxdBootWaitOpts) readinessConfig() *lxd.ReadinessConfig {
	config := lxd.DefaultReadinessConfig()
	if b == nil {
		return config
	}
	if b.Timeout > 0 {
		config.Timeout = time.Duration(b.Timeout) * time.Second
	}
	if b.Interval > 0 {
		config.PollInterval = time.Duration(b.Interval) * time.Second
	}
	return config
}

// readyCommand returns the command used to determine whether the instance is ready,
// wrapped in the shell configured for the node.
func (b *LxdBootWaitOpts) readyCommand(shell string) []string {
	command := "true"
	if b != nil && b.ReadyCommand != "" {
		command = b.ReadyCommand
	}
	return []string{shell, "-c", command}
}

// optionValueToString renders a YAML scalar in the string form that the LXD API expects.
func optionValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildDevices converts the configured devices into the string map that LXD expects.
// Relative disk sources are resolved against the working directory so that test files
// can reference build artifacts by their path in the repository. Sources are paths on
// the LXD host, so they are left untouched when the node talks to a remote server.
func buildDevices(devices map[string]map[string]interface{}, resolvePaths bool) (map[string]map[string]string, error) {
	built := make(map[string]map[string]string, len(devices))
	for deviceName, device := range devices {
		converted := make(map[string]string, len(device))
		for key, value := range device {
			converted[key] = optionValueToString(value)
		}

		if converted["type"] == "" {
			return nil, helpers.WrapError(fmt.Sprintf("device %q is missing a type", deviceName))
		}

		// Only plain disks reference a host path; disks backed by a storage pool name a volume
		if resolvePaths && converted["type"] == "disk" && converted["pool"] == "" && converted["source"] != "" {
			absolute, err := filepath.Abs(converted["source"])
			if err != nil {
				return nil, helpers.WrapError(fmt.Sprintf("device %q: unable to resolve source %q: %v", deviceName, converted["source"], err))
			}
			converted["source"] = absolute
		}

		built[deviceName] = converted
	}

	return built, nil
}

// NewLxdNode creates a new LXD node without using the wrapper
func NewLxdNode(name string, opts ifaces.NodeOptions) (node ifaces.Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts LxdNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	if err = nodeopts.validate(); err != nil {
		return nil, err
	}

	// Set defaults
	if nodeopts.Server == "" {
		nodeopts.Server = "local"
	}
	if nodeopts.Protocol == "" {
		nodeopts.Protocol = "lxd"
	}
	if nodeopts.InstanceType == "" {
		nodeopts.InstanceType = "container"
	}
	if nodeopts.Project == "" {
		nodeopts.Project = lxd.DefaultProject
	}

	// Detect runtime for local connections (needed for image translation)
	var detectedRuntime platform.Runtime = platform.RuntimeLXD
	if nodeopts.RemoteAddr == "" && nodeopts.Socket == "" {
		// Auto-detect LXD vs Incus
		result, err := platform.DetectRuntime()
		if err != nil {
			return nil, fmt.Errorf("failed to detect container runtime: %w", err)
		}
		nodeopts.Socket = result.SocketPath
		detectedRuntime = result.Runtime
	} else if nodeopts.Socket == "/var/lib/incus/unix.socket" {
		detectedRuntime = platform.RuntimeIncus
	}

	// Translate image name for Incus if needed (before parsing remote:alias)
	if detectedRuntime == platform.RuntimeIncus && strings.Contains(nodeopts.Image, ":") {
		nodeopts.Image = platform.TranslateImage(nodeopts.Image, detectedRuntime)
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

	// Connect to LXD server (local or remote)
	var client lxdclient.InstanceServer
	if nodeopts.RemoteAddr != "" {
		// Remote LXD server connection

		// Check authentication method priority: trust_token > certificates > skip_verify
		if nodeopts.TrustToken != "" {
			// Use trust token authentication (modern approach)
			// Generate temporary certificates for the initial connection
			certPEM, keyPEM, err := generateSelfSignedCert()
			if err != nil {
				return nil, fmt.Errorf("failed to generate temporary certificate for token auth: %w", err)
			}

			// Connect with generated certificates (not yet trusted)
			args := &lxdclient.ConnectionArgs{
				TLSClientCert: certPEM,
				TLSClientKey:  keyPEM,
			}

			// For token-based authentication, we must skip verification initially since we're not yet
			// in the trust store. However, if a server certificate is provided, we can validate against it.
			if nodeopts.ServerCert != "" {
				// Server cert provided - validate against it
				args.TLSServerCert = nodeopts.ServerCert
				args.InsecureSkipVerify = false
			} else {
				// No server cert - must skip verification for initial connection
				// This is acceptable for token-based auth as the token itself provides authentication
				args.InsecureSkipVerify = true
			}

			client, err = lxdclient.ConnectLXD(nodeopts.RemoteAddr, args)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to remote LXD server at %s for token auth: %w", nodeopts.RemoteAddr, err)
			}

			// Authenticate using the trust token
			clientName := fmt.Sprintf("dart-%s", name)
			err = authenticateWithToken(client, nodeopts.TrustToken, clientName)
			if err != nil {
				return nil, fmt.Errorf("failed to authenticate with trust token: %w", err)
			}

		} else {
			// Use certificate-based authentication
			// Validate that if remote connection is configured, proper authentication is provided
			if nodeopts.ClientCert == "" || nodeopts.ClientKey == "" {
				if !nodeopts.SkipVerify {
					return nil, fmt.Errorf("remote LXD connection requires either trust_token OR (client_cert AND client_key), or set skip_verify: true (not recommended for production)")
				}
			}

			// Connect to remote LXD server using HTTPS
			args := &lxdclient.ConnectionArgs{
				InsecureSkipVerify: nodeopts.SkipVerify,
			}

			// Set client certificate and key if provided
			if nodeopts.ClientCert != "" && nodeopts.ClientKey != "" {
				args.TLSClientCert = nodeopts.ClientCert
				args.TLSClientKey = nodeopts.ClientKey
			}

			// Set server certificate if provided (for custom CA)
			if nodeopts.ServerCert != "" {
				args.TLSServerCert = nodeopts.ServerCert
			}

			client, err = lxdclient.ConnectLXD(nodeopts.RemoteAddr, args)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to remote LXD server at %s: %w", nodeopts.RemoteAddr, err)
			}
		}
	} else {
		// Connect to local LXD server using Unix socket
		// Use the specified socket path, or empty string for system default
		socketPath := nodeopts.Socket
		client, err = lxdclient.ConnectLXDUnix(socketPath, nil)
		if err != nil {
			return nil, err
		}
	}

	// Use the specified project (or default)
	if nodeopts.Project != lxd.DefaultProject {
		client = client.UseProject(nodeopts.Project)
	}

	return &LxdNode{
		name:    name,
		options: nodeopts,
		client:  client,
	}, nil

}

// NewLxdNodeWithWrapper creates a new LXD node using the provided wrapper
// Note: When using a wrapper, the connection to the LXD server is managed by the wrapper itself.
// Remote connection configuration in node options will be ignored.
// Use NewWrapper or NewWrapperWithOptions to configure remote connections when using wrappers.
func NewLxdNodeWithWrapper(wrapper *lxd.Wrapper, name string, opts ifaces.NodeOptions) (node ifaces.Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts LxdNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	if err = nodeopts.validate(); err != nil {
		return nil, err
	}

	// Set defaults
	if nodeopts.Server == "" {
		nodeopts.Server = "local"
	}
	if nodeopts.Protocol == "" {
		nodeopts.Protocol = "lxd"
	}
	if nodeopts.InstanceType == "" {
		nodeopts.InstanceType = "container"
	}
	if nodeopts.Project == "" {
		nodeopts.Project = lxd.DefaultProject
	}

	// Translate image name for Incus if needed (before parsing remote:alias)
	wrapperRuntime := wrapper.GetRuntime()
	if wrapperRuntime == platform.RuntimeIncus && strings.Contains(nodeopts.Image, ":") {
		nodeopts.Image = platform.TranslateImage(nodeopts.Image, wrapperRuntime)
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

	// Get the server from wrapper and use the project if specified
	client := wrapper.GetServer()
	if nodeopts.Project != lxd.DefaultProject {
		client = client.UseProject(nodeopts.Project)
	}

	return &LxdNode{
		name:    name,
		options: nodeopts,
		wrapper: wrapper,
		client:  client,
	}, nil
}

type LxdNode struct {
	name      string
	client    lxdclient.InstanceServer
	wrapper   *lxd.Wrapper
	options   LxdNodeOpts
	addresses []string
}

func (d *LxdNode) Setup() error {
	if d.client == nil {
		return helpers.WrapError("lxd client not initialized")
	}

	// Determine the instance type
	instanceType := api.InstanceType(d.options.InstanceType)

	// Build network devices from the options.Networks configuration
	// Use eth0, eth1, etc. to override profile NICs (default profile typically has eth0)
	devices := make(map[string]map[string]string)
	for i, netOpts := range d.options.Networks {
		// Use eth0, eth1, etc. naming to override default profile NIC devices
		deviceName := fmt.Sprintf("eth%d", i)
		deviceConfig := map[string]string{
			"type":    "nic",
			"network": netOpts.Name,
		}
		// Add static IP address if specified, detecting IPv4 vs IPv6
		if netOpts.Ip != "" {
			ip := net.ParseIP(netOpts.Ip)
			if ip == nil {
				return helpers.WrapError(fmt.Sprintf("invalid IP address for network %s: %s", netOpts.Name, netOpts.Ip))
			}
			if ip.To4() != nil {
				deviceConfig["ipv4.address"] = netOpts.Ip
			} else {
				deviceConfig["ipv6.address"] = netOpts.Ip
			}
		}
		devices[deviceName] = deviceConfig
	}

	// Merge in any explicitly configured devices, such as an ISO attached as boot media.
	// These are applied last so a node can override a generated NIC if it needs to.
	configuredDevices, err := buildDevices(d.options.Devices, d.options.RemoteAddr == "")
	if err != nil {
		return err
	}
	for deviceName, deviceConfig := range configuredDevices {
		devices[deviceName] = deviceConfig
	}

	// Build the instance configuration keys
	instanceConfig := make(map[string]string, len(d.options.Config))
	for key, value := range d.options.Config {
		instanceConfig[key] = optionValueToString(value)
	}

	// Empty instances are created without a source so they boot from their devices
	source := api.InstanceSource{
		Type:     api.SourceTypeImage,
		Alias:    d.options.Image,
		Server:   d.options.Server,
		Protocol: d.options.Protocol,
	}
	if d.options.emptyInstance() {
		source = api.InstanceSource{Type: api.SourceTypeNone}
	}

	// Create a request for the instance
	req := api.InstancesPost{
		Name:   d.name,
		Source: source,
		Type:   instanceType,
		InstancePut: api.InstancePut{
			Profiles: d.options.Profiles,
			Config:   instanceConfig,
			Devices:  devices,
		},
	}

	op, err := d.client.CreateInstance(req)
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error creating instance: %v", err))
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error creating instance: %v", err))
	}

	// Start the instance
	reqState := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err = d.client.UpdateInstanceState(d.name, reqState, "")
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error starting instance: %v", err))
	}

	if err := op.Wait(); err != nil {
		return helpers.WrapError(fmt.Sprintf("error starting instance: %v", err))
	}

	return d.waitForReady()
}

// waitForReady blocks until the instance can run commands. When boot_wait is configured
// the default readiness check is replaced by a poll of the configured command, which is
// what an instance installing from an ISO needs: it is unreachable while the installer
// runs and only answers once it has rebooted from disk.
func (d *LxdNode) waitForReady() error {
	ctx := context.Background()

	if d.options.BootWait != nil {
		if delay := d.options.BootWait.InitialDelay; delay > 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
		command := d.options.BootWait.readyCommand(d.shell())
		if err := lxd.WaitForInstanceCommand(ctx, d.client, d.name, command, d.options.BootWait.readinessConfig()); err != nil {
			return helpers.WrapError(fmt.Sprintf("error waiting for instance to be ready: %v", err))
		}
		return nil
	}

	// An empty instance has no guest agent until something is installed into it, so
	// there is nothing to wait for unless the node asked for it with boot_wait
	if d.options.emptyInstance() {
		return nil
	}

	// Wait for the instance to be fully ready (OS booted, networking available)
	if err := lxd.WaitForInstanceReady(ctx, d.client, d.name, nil); err != nil {
		return helpers.WrapError(fmt.Sprintf("error waiting for instance to be ready: %v", err))
	}

	return nil
}

// shell returns the shell used to run commands inside the instance
func (d *LxdNode) shell() string {
	if shell, ok := d.options.ExecOptions["shell"].(string); ok && shell != "" {
		return shell
	}
	return "/bin/bash"
}

func (d *LxdNode) Teardown() error {
	if d.client == nil {
		return helpers.WrapError("lxd client not initialized")
	}

	// An instance may already be stopped, for example a VM that powered itself off at
	// the end of an unattended install, and stopping it again is an error
	state, _, err := d.client.GetInstanceState(d.name)
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error getting instance state: %v", err))
	}

	var op lxdclient.Operation
	if state.Status != "Stopped" {
		// Create a stop request
		req := api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
			Force:   true,
		}
		op, err = d.client.UpdateInstanceState(d.name, req, "")
		if err != nil {
			return helpers.WrapError(fmt.Sprintf("error stopping instance: %v", err))
		}
		if err = op.Wait(); err != nil {
			return helpers.WrapError(fmt.Sprintf("error stopping instance: %v", err))
		}
	}

	// Create a delete request
	op, err = d.client.DeleteInstance(d.name)
	if err != nil {
		return helpers.WrapError(fmt.Sprintf("error deleting instance: %v", err))
	}
	if err = op.Wait(); err != nil {
		return helpers.WrapError(fmt.Sprintf("error deleting instance: %v", err))
	}

	return nil
}

func (d *LxdNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {

	if d.client == nil {
		return nil, helpers.WrapError("lxd client not initialized")
	}

	debugEnabled := execution.IsDebugMode()

	// Create TeeWriters that optionally stream to console
	stdoutWriter := stream.NewTeeWriter(stream.StreamStdout, d.name, debugEnabled)
	stderrWriter := stream.NewTeeWriter(stream.StreamStderr, d.name, debugEnabled)

	execArgs := lxdclient.InstanceExecArgs{
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
	}

	// Execute the command using the configured shell
	execPost := api.InstanceExecPost{
		Command:     []string{d.shell(), "-c", command},
		WaitForWS:   true,
		Interactive: false,
	}

	op, err := d.client.ExecInstance(d.name, execPost, &execArgs)
	if err != nil {
		return nil, helpers.WrapError(fmt.Sprintf("error executing command: %v", err))
	}

	if err = op.Wait(); err != nil {
		return nil, helpers.WrapError(fmt.Sprintf("error executing command: %v", err))
	}

	metadata := op.Get().Metadata
	exitCode, ok := metadata["return"].(float64)
	if !ok {
		return nil, helpers.WrapError("error getting exit code")
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    int(exitCode),
		Stdout:      stdoutWriter.Reader(),
		Stderr:      stderrWriter.Reader(),
	}, nil
}

func (d *LxdNode) Close() error {
	// No cleanup needed for the LXD client
	return nil
}

// generateSelfSignedCert generates a self-signed certificate for LXD client authentication
func generateSelfSignedCert() (certPEM, keyPEM string, err error) {
	// Generate private key (2048-bit RSA provides good security/performance balance)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "dart-lxd-client",
			Organization: []string{"DART"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEMBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}
	certPEMBytes := pem.EncodeToMemory(certPEMBlock)

	// Encode private key to PEM
	keyPEMBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	keyPEMBytes := pem.EncodeToMemory(keyPEMBlock)

	return string(certPEMBytes), string(keyPEMBytes), nil
}

// authenticateWithToken authenticates with LXD server using a trust token
func authenticateWithToken(server lxdclient.InstanceServer, token, clientName string) error {
	// Generate a self-signed certificate for authentication
	certPEM, _, err := generateSelfSignedCert()
	if err != nil {
		return fmt.Errorf("failed to generate client certificate: %w", err)
	}

	// Encode certificate to base64
	certBase64 := base64.StdEncoding.EncodeToString([]byte(certPEM))

	// Create certificate request with trust token
	certReq := api.CertificatesPost{
		Name:        clientName,
		Type:        "client",
		Certificate: certBase64,
		TrustToken:  token,
	}

	// Send certificate to server with trust token
	err = server.CreateCertificate(certReq)
	if err != nil {
		return fmt.Errorf("failed to authenticate with trust token: %w", err)
	}

	return nil
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
