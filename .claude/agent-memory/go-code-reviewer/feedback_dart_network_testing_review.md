---
name: feedback-dart-network-testing-review
description: Verified bugs found reviewing feature/network-testing (port_check from:node, tls_cert, auto network facts) — shell injection, busybox nc -z gap, IP-SAN false negative.
metadata:
  type: project
---

Branch `feature/network-testing` (single commit "WIP batch D" on top of
`feature/vars-tags`) implements #72/#76/#77: node-side `port_check`
(`from: node`), `tls_cert` test type, and auto network facts
(`NetworkInspector`/`NetworkFacts` for lxd/docker). See
[[project-dart-overview]] for general repo layout.

## Verified bugs

1. **Shell command injection in `port_check` `from: node`**
   (`pkg/testtypes/port_check.go:101-103`). The generated probe quotes
   `host` with `helpers.ShellQuote` for the `nc` branch but interpolates it
   **raw** into the `/dev/tcp/%s/%d` fallback inside a single-quoted
   `bash -c '...'` string. A host value containing a single quote breaks
   out of that quoting and injects arbitrary shell commands into the
   *outer* script (the one `node.Execute` passes to `/bin/sh -c`, since
   `LocalNode`/lxd/docker all shell out with the whole probe string as one
   `-c` argument — see `pkg/nodetypes/local.go:26` comment). Verified
   end-to-end via a real `newPortCheckTest` + `LocalNode` + forced
   no-`nc`-in-PATH repro: `host: "x'; touch MARKER; echo '"` executes
   `touch MARKER`. Reachable in practice because `host` is often supplied
   via templating that doesn't character-filter: `internal/facts.go`
   `RenderTemplate`'s `fact()` function inserts fact-command stdout
   verbatim (no quote filtering at all — facts run arbitrary node
   commands), and `internal/config/config.go`'s new `yamlRiskyChars` guard
   (added in the same commit) only blocks *unquoted* risky chars in
   `{{var.x}}`/`{{env.X}}` substitution — a double-quoted YAML position
   (`host: "{{var.target}}"`) is allowed straight through with a `'` intact.
   Fix direction: `helpers.ShellQuote` both occurrences of `host` (and
   `port` for defense-in-depth even though it's int-validated 1-65535).

2. **`nc -z` silently unsupported on minimal/busybox `nc` builds — probe
   always reports "closed"** (`pkg/testtypes/port_check.go:101-103`,
   probe logic branches only on `command -v nc`, never on flag support).
   Verified with the box's own `busybox nc` (`BusyBox v1.37.0`, the
   default/minimal applet build without `FEATURE_NC_EXTRA`, common in
   slim/embedded container images): `nc -z -w 3 host port` → `nc: invalid
   option -- 'z'`, exit 1. Since `command -v nc` succeeds, DART's `if`
   branch is taken and always hits this — the compound's exit status is
   always 1 regardless of true port state, so `echo closed` always fires.
   This silently produces false negatives (or false "closed" positives)
   on any container base with a minimal busybox `nc` and no `-z`/`-v`
   support — a realistic base for the kind of containers DART tests
   against. Compounds with the fact that the `/dev/tcp` fallback assumes
   `bash` is present, which is not true on stock Alpine/ash-only images
   either — so "no nc with -z, no bash" (a plausible minimal-image
   combination) makes `from: node` port_check silently always-wrong with
   no error surfaced.

3. **`tls_cert` `dns_names` check false-negatives on IP SAN coverage**
   (`pkg/testtypes/tls_cert.go:38-46` `certFacts` struct has no
   `IPAddresses`/`ip_addresses` field; `certNamesCheck.Verify` at
   `tls_cert.go:254` reconstructs `&x509.Certificate{DNSNames:
   facts.DNSNames}` — IP SAN info is dropped during the JSON round-trip
   and never reaches the reconstructed cert). Verified: a cert with only
   an IP SAN (no DNSNames) for `127.0.0.1` genuinely verifies via
   `leaf.VerifyHostname("127.0.0.1")` (ground truth, using the real leaf
   cert — Go's `VerifyHostname` checks `IPAddresses` when the name is an
   IP literal), but the same check against the reconstructed
   DNSNames-only certificate fails with "doesn't contain any IP SANs".
   Internal-service certs commonly use IP SANs; a `dns_names` check
   against one produces a false failure with no workaround (no separate
   `ip_addresses` evaluator exists).

## Checked and confirmed NOT bugs (don't re-investigate)

- `Dialer.Timeout` in `TLSCertTest.inspect` (`tls_cert.go:166-167`) DOES
  bound the handshake, not just the dial — Go 1.26's
  `crypto/tls.dial()` wraps `DialContext` and `HandshakeContext` in the
  same timeout-derived `context.Context`.
- IP-literal `ServerName`/SNI doesn't break the handshake — Go's
  `hostnameInSNI` omits the SNI extension for IP literals per RFC 6066;
  handshake proceeds normally without it.
- `conn.Close()` on the "no peer certificates" early-return path in
  `tls_cert.go` — `defer conn.Close()` is registered before that check,
  no leak.
- Facts are gathered strictly after node `Setup()` completes and before
  step/test creation (`internal/controller.go:382-426`) — no ordering bug.
- Built-in vs. user fact name collision — user facts genuinely win; the
  builtins loop populates `store[cfg.Name]` first, the user-facts loop
  fetches the same map (by reference) and overwrites by name
  (`internal/facts/facts.go:26-92`).
- LXD `NetworkFacts` interface-name iteration is sorted
  (`pkg/nodetypes/lxd.go:542-546`) — deterministic.
- `port`/`port_check` and `tls_cert` both validate `1 <= port <= 65535`
  before use — not exploitable via negative/huge values.

## Minor / worth a follow-up look

- `port_check.go:97-100`: `timeoutSecs := int(timeoutSeconds)` truncates
  (not rounds) fractional `timeout:` values before clamping to a 1s
  floor, so `timeout: 2.9` becomes a 2s node-side probe timeout while
  `from: host` mode would honor 2.9s exactly — same option, different
  precision semantics depending on `from`.
- `internal/facts/facts.go` `HasAnyFacts` (`facts.go:257-267`) returns
  true for *any* node implementing `NetworkInspector` (i.e. every
  lxd/docker node), not just when facts are actually used — every
  existing docker/lxd suite now gets an unrequested "Gathering node
  facts" header + a `StartTask`/`Complete` line per built-in address fact
  in its console output (`internal/controller.go:406-423`), a behavior
  change invisible to unit tests since `MockNode` doesn't implement
  `NetworkInspector`.

## Verification method

Same as prior reviews on this repo (see [[feedback-dart-retry-timeout-review]]):
write a throwaway `_test.go` file that drives the *real* production code
path (not hand-rolled string checks), run it, delete it. For the shell
injection: used `busybox`/PATH manipulation to force the vulnerable
`/dev/tcp` fallback branch (the box's own `nc` normally wins). For the
busybox `-z` gap: this box's own `/usr/bin/busybox nc` already lacks
`FEATURE_NC_EXTRA`, so no container was needed — ran it directly.
