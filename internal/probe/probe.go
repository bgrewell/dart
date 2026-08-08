// Package probe builds the shell commands that let network checks run on
// the node a test or step names, and reshapes their output into the same
// form the controller-side implementations produce.
//
// Rationale: a suite that says `node: web-01` is asking what web-01 can
// see. The controller sits in a different network namespace with different
// routes, resolvers, and firewall rules, so a probe run there answers a
// different question — and answers it wrongly often enough to matter.
package probe

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/helpers"
)

// Vantage selects where a network probe is performed.
type Vantage string

const (
	// VantageNode probes from the node the test or step names. Default.
	VantageNode Vantage = "node"
	// VantageHost probes from the machine running DART, for the cases that
	// genuinely want the controller's viewpoint (a published port, an
	// endpoint that must be reachable from CI).
	VantageHost Vantage = "host"
)

// ParseVantage validates a raw `from` value. An empty value means the
// option was not set and the default applies.
func ParseVantage(raw string) (Vantage, bool) {
	if raw == "" {
		return VantageNode, true
	}
	switch Vantage(raw) {
	case VantageNode, VantageHost:
		return Vantage(raw), true
	default:
		return "", false
	}
}

// RequireTool builds a shell guard that fails loudly with exit 127 when a
// probe's dependency is missing on the node, rather than letting an absent
// binary read as a failed check.
func RequireTool(tool string) string {
	return fmt.Sprintf(
		`command -v %s >/dev/null 2>&1 || { echo "dart: %s is required on this node for this check" >&2; exit 127; }`,
		helpers.ShellQuote(tool), tool)
}

// MissingToolExitCode is the exit code RequireTool uses, matching the
// shell's own convention for "command not found".
const MissingToolExitCode = 127

// httpStatusSentinel separates the response body from the status code that
// curl appends. It is deliberately unlikely to occur in a payload.
const httpStatusSentinel = "\n__dart_http_status__"

// HTTPCommand builds the shell command that performs an HTTP request on the
// node. Header values and the URL are shell-quoted rather than interpolated
// raw, so values arriving from facts, vars, or captures cannot execute.
func HTTPCommand(method, url string, headers map[string]string, timeoutSeconds float64) string {
	args := []string{
		"--silent", "--show-error", "--location",
		"-X", helpers.ShellQuote(method),
		"--max-time", fmt.Sprintf("%.0f", math.Ceil(timeoutSeconds)),
		"-w", helpers.ShellQuote(httpStatusSentinel + "%{http_code}"),
	}

	// Deterministic header order keeps the generated command stable
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		args = append(args, "-H", helpers.ShellQuote(name+": "+headers[name]))
	}
	args = append(args, helpers.ShellQuote(url))

	return RequireTool("curl") + "\ncurl " + strings.Join(args, " ")
}

// ParseHTTPOutput splits the probe's stdout into the response body and the
// HTTP status code. A missing sentinel means curl never got a response —
// DNS failure, connection refused, or timeout — in which case stderr
// carries the useful message.
func ParseHTTPOutput(stdout, stderr string) (body string, status int, err error) {
	index := strings.LastIndex(stdout, httpStatusSentinel)
	if index < 0 {
		message := strings.TrimSpace(stderr)
		if message == "" {
			message = strings.TrimSpace(stdout)
		}
		return "", 0, fmt.Errorf("request failed on node: %s", message)
	}

	status, err = strconv.Atoi(strings.TrimSpace(stdout[index+len(httpStatusSentinel):]))
	if err != nil {
		return "", 0, fmt.Errorf("could not read the status code from the node's response: %w", err)
	}
	if status == 0 {
		// curl writes the sentinel even when no transfer completed, with
		// http_code 000 — a TLS verification failure, a reset, a redirect
		// loop. Its own message is the only useful diagnostic, and the
		// host-side client reports these as errors too.
		message := strings.TrimSpace(stderr)
		if message == "" {
			message = "no response"
		}
		return "", 0, fmt.Errorf("request failed on node: %s", message)
	}
	return stdout[:index], status, nil
}

// DNSCommand builds a resolver-agnostic lookup that prints one address per
// line. The tools are tried in order of how faithfully they reflect what
// the node's own applications would resolve: getent consults nsswitch (so
// /etc/hosts and any resolver plugins count), and the DNS-only tools are
// fallbacks for images that ship none of it.
func DNSCommand(hostname string) string {
	quoted := helpers.ShellQuote(hostname)
	return fmt.Sprintf(`if command -v getent >/dev/null 2>&1; then
  getent ahosts %[1]s | awk '{print $1}' | sort -u
elif command -v dig >/dev/null 2>&1; then
  { dig +short A %[1]s; dig +short AAAA %[1]s; } | grep -v '\.$'
elif command -v host >/dev/null 2>&1; then
  host %[1]s | awk '/has address|has IPv6 address/ {print $NF}'
elif command -v nslookup >/dev/null 2>&1; then
  nslookup %[1]s 2>/dev/null | sed -n '/^Name:/,$p' | awk '/^Address/ {print $NF}'
else
  echo "dart: one of getent, dig, host, or nslookup is required on this node for this check" >&2
  exit %[2]d
fi`, quoted, MissingToolExitCode)
}

// ParseAddresses reads DNSCommand's output. Addresses are returned in the
// order the node reported them, deduplicated.
func ParseAddresses(stdout string) []string {
	seen := map[string]bool{}
	addresses := make([]string, 0, 4)
	for _, line := range strings.Split(stdout, "\n") {
		address := strings.TrimSpace(line)
		// nslookup renders answers as "Address: 1.2.3.4"; some resolvers
		// append a port
		address = strings.TrimPrefix(address, "Address:")
		address = strings.TrimSpace(address)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		addresses = append(addresses, address)
	}
	return addresses
}

// TLSCommand fetches a peer certificate chain from the node in PEM form.
// The certificates are parsed on the controller so node-side and host-side
// checks evaluate identically. Note: the handshake is bounded by the
// caller's command timeout, not by openssl, whose own timeout flags do not
// cover a stalled TCP connect.
func TLSCommand(host string, port int, serverName string) string {
	return RequireTool("openssl") + fmt.Sprintf(
		"\nopenssl s_client -connect %s:%d -servername %s -showcerts </dev/null 2>/dev/null",
		helpers.ShellQuote(host), port, helpers.ShellQuote(serverName))
}
