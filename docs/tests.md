# Test Types

Tests run against a node and evaluate the outcome. This is the full reference for every test type and the behaviours shared across them (retries, skips, captures, variables).

[← Back to the README](../README.md)

`execute` is the general-purpose type; the others are shorthand for common
checks and accept the same `evaluate` keys where noted.

| Type | What it does | Key options |
|------|--------------|-------------|
| `execute` | Run a command on the node, evaluate its result | `command`; `evaluate` (see [Test Evaluation Reference](evaluation.md)) |
| `exists` | Check a path exists on the node (`test -e`) | `path` (alias `filename`); `evaluate.exists: true\|false` (default `true`) |
| `file_content` | Read a file on the node with `cat`, evaluate its content | `filename` (alias `path`); standard `evaluate` keys apply to the content |
| `file_hash` | Verify file checksums on the node | `filename` (alias `path`); `evaluate.md5/sha1/sha256` hex digests (at least one required) |
| `service_status` | Check a systemd unit state on the node | `service`; `evaluate.status` (default `active`) |
| `ping` | Ping a target from the node | `target` (alias `host`), `count` (default 5); `evaluate.packet_loss` (max %), `rtt_avg`/`rtt_max` (ms, upper bounds), `rtt_min` (ms, **lower** bound) |
| `http_request` | HTTP request from the node, or from the DART host (no request body — see below) | `url` (required), `method` (default `GET`, upper-cased), `headers`, `timeout` (seconds, default 30), `from: node\|host` (default `node`); `evaluate.status_code` (one integer) plus standard keys against the response body |
| `port_check` | TCP connect to `host:port`, from the node or from the DART host | `host`, `port` (both required); `from: node\|host` (default `node`), `timeout` (seconds, default 5); `evaluate.status: open\|closed` (default `open`) |
| `reboot` | Restart the node mid-suite and wait until it accepts commands | `mode: graceful\|force`, `ready_command`, `timeout` (lxd and ssh nodes) |
| `consistency` | Compare one command's output **across** nodes (two or more) | `command`, `nodes` (optional subset of `node:`), `timeout`; `evaluate.all_equal`, `matching: {pattern, count}` (`count` defaults to 1) |
| `tls_cert` | Inspect a TLS endpoint's certificate, from the node or from the DART host | `host`, `port` (443), `server_name` (defaults to `host`), `timeout` (seconds, default 10), `from: node\|host` (default `node`); `evaluate.min_days_remaining`, `dns_names`, `issuer_contains`, `subject_contains`, `chain_valid` |

Four types accept an alias for their path or target option: `exists` takes
`path` or `filename`, `file_content` and `file_hash` take `filename` or
`path`, and `ping` takes `target` or `host`. The first name listed is the
primary one and is what a missing-option error reports (`path is required in
test "…"`). No other type has a second accepted spelling — `port_check` and
`tls_cert` accept `host` only, `http_request` accepts `url` only,
`service_status` accepts `service` only, and `execute` and `consistency`
accept `command` only.

Any test also accepts test-level `retry:` (see Timeouts and Retries),
`skip_if`/`skip_unless` (see Conditional Skips), and `setup:`/`teardown:`
command lists (see Per-Test Setup and Teardown).

Every test must name a `node:`; a test without one fails configuration
loading with `test "<name>" references no node`. As with steps, `node:`
accepts a single name or a list, and a list expands into one test per node in
the order listed — the sole exception is `consistency`, described under
Cluster Consistency.

The standard `evaluate` keys shared by these types — `exit_code`,
`exit_code_not`, `match`, `stderr_match`, `contains`, `not_contains`,
`stderr_contains`, `regex`, `stderr_regex`, `empty`, `stderr_empty`,
`line_count`, `gt`, `lt`, `ge`, `le`, `max_duration`, and `json_path` — are
documented in the [Test Evaluation Reference](evaluation.md). Every listed
check must pass for the test to pass; checks are evaluated and reported in
alphabetical order; a test with no checks is reported as `ran` rather than
passed. Note that `gte`/`lte` are not registry evaluators — they are accepted
only inside the comparator maps described under Value Extraction and Numeric
Assertions.

An unrecognized key inside `options:` is a configuration error naming the
offending key and the full accepted set, caught by `--check` before anything
is created:

```text
Error: unknown option "evaluatte" in test "service responds"
(an execute test accepts: capture, command, evaluate, extract, timeout)
```

That covers misspelled option names (`timout` for `timeout`), options that
exist only on another type (`capture:` and `extract:` are honoured by
`execute` only), and — the most damaging form — an evaluator written at the
option level rather than nested inside `evaluate:`:

```yaml
    options:
      command: systemctl is-active nginx
      exit_code: 0        # error: belongs inside evaluate:
```

Rationale: a dropped `evaluate` block left the test with zero checks, which
is reported as `ran` rather than passed or failed, so the suite exited 0
having asserted nothing.

Keys *inside* `evaluate:` are checked the same way against the evaluator
registry, so `exit_cod: 0` fails with `unknown evaluation type "exit_cod"`.
Missing *required* options (`command is required in test "t"`) and
unrecognised `type:` values are caught alongside them.

Warning: one gap remains. Keys at the *test* level rather than inside
`options:` — a stray key next to `name:`/`node:`/`type:`, or a misspelled
top-level `test:` instead of `tests:`, which yields a suite with zero tests —
are still dropped by the YAML decoder before any of this runs. The suite
summary printed by `--check` is what catches those: a test count lower than
expected means a test never parsed.

### Default Checks

Test types that stand for a specific question inject a check of their own.
Two rules apply, depending on the type.

**Always applied, on top of anything else in `evaluate`.** For `exists`,
`service_status`, and `port_check` the signature check is part of the test,
not a default that other keys replace. The corresponding `evaluate` key only
changes what the check expects:

| Type | Check | Key | Default |
|------|-------|-----|---------|
| `exists` | path exists on the node | `exists: true\|false` | `true` |
| `service_status` | `systemctl is-active` output matches exactly | `status: <state>` | `active` |
| `port_check` | observed TCP state matches | `status: open\|closed` | `open` |

An `exists` test with `evaluate: {contains: "x"}` therefore asserts both that
the path exists *and* that the output contains `x`; a `port_check` with only
`contains:` still asserts the port is open.

**Applied only when no `evaluate` keys are given.** For the types below the
default is a fallback: an `evaluate` block containing *any* key — including an
unrelated one — replaces it entirely.

| Type | Default check when `evaluate` is omitted or empty |
|------|--------------------------------------------------|
| `ping` | `packet_loss: 0` |
| `http_request` | `status_code: 200` |
| `tls_cert` | `min_days_remaining: 0` (certificate not expired) |
| `consistency` | `all_equal: true` |
| `file_content` | `readable` — the file is readable (`cat` exits 0) |
| `reboot` | `rebooted` — the readiness command exits 0 |

Warning: `ping` with `evaluate: {rtt_max: 10}` does **not** assert zero packet
loss, and `http_request` with `evaluate: {contains: "ok"}` does **not** assert
a 200. List the default key explicitly whenever other keys are present.

`file_hash` has no default: at least one of `md5`/`sha1`/`sha256` is required,
and omitting them is a configuration error.

```yaml
tests:
  - name: reboot to apply rollback
    node: iso-vm
    type: reboot
    options:
      mode: force            # model a power cut; graceful is the default
      ready_command: cat /etc/hostname
      timeout: 600
```

`reboot` is also available as a setup/teardown step with the same options.
`mode: force` restarts without a clean shutdown, which is what crash-safety
suites need. `timeout` is in seconds for both forms and must be non-negative.

On LXD nodes the readiness wait starts from the node's `boot_wait`
configuration, and the test's options override it rather than adding to it:
`ready_command` replaces `boot_wait.ready_command` (run through the node's
configured shell) and `timeout` replaces `boot_wait.timeout`. With no
`boot_wait` on the node the defaults are a 5-minute timeout and a 2-second
poll interval; the poll interval comes only from `boot_wait.interval` and
cannot be set on the reboot test or step.

On SSH nodes the reboot is issued over the session (`sudo -n reboot`, falling
back to a direct `reboot` for root sessions; `mode: force` adds `-f`) and DART
then reconnects until the host answers. An omitted or zero `timeout` waits up
to 5 minutes. DART sleeps a fixed 5 seconds before the first probe, so the
poll cannot succeed against the still-running pre-reboot host, then redials
and runs `ready_command` every 3 seconds. Without `ready_command` the check is
`true`, meaning any SSH session that opens and runs a command counts as ready
— a `ready_command` that only passes once the services under test are back up
is the way to make the wait meaningful.

```yaml
tests:
  - name: API serves healthy status
    node: app-server
    type: http_request
    options:
      url: http://localhost:8080/health
      evaluate:
        status_code: 200     # required once any other evaluate key is present
        json_path:
          path: status
          equals: healthy

  - name: config deployed with right hash
    node: app-server
    type: file_hash
    options:
      filename: /etc/myapp/config.ini
      evaluate:
        sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

  - name: database reachable with low latency
    node: app-server
    type: ping
    options:
      target: db.internal
      count: 5
      evaluate:
        packet_loss: 0
        rtt_max: 10
```

### `http_request` Details

The response status code becomes the result's exit code and the body its
stdout, so `status_code`, `contains`, `match`, `regex`, `json_path`, and
`max_duration` all apply. `status_code: 200` is assumed **only when
`evaluate` is omitted or empty**. As soon as `evaluate` contains any key, the
status code is no longer checked unless `status_code` is listed explicitly —
`evaluate: {contains: healthy}` alone passes on a 500 whose error page happens
to contain `healthy`. `method` is upper-cased before the request is made, and
`timeout` is in seconds (fractional values allowed); a non-positive `timeout`
is a config error. Note: under the default `from: node` the value reaches
`curl --max-time` rounded **up** to whole seconds, so `timeout: 0.5` is a
one-second bound rather than 500 ms, and DART additionally bounds the command
suite-side at `timeout + 5` so a stalled transport cannot hang the run.

`status_code` takes exactly one integer; a list such as `[200, 204]` is a
config error. Because the response status is exposed as the result's exit
code, a set of acceptable statuses is written with the standard exit-code
checks instead — `exit_code: [200, 204]`, or `exit_code_not: 500` to reject
specific ones.

`http_request` covers reachability and response assertions; it is
deliberately not a full HTTP client.

- **No request body.** The request is always sent with an empty body
  regardless of `method`, so `POST`/`PUT`/`PATCH` reach the server with
  nothing attached. There is no `body`, `data`, or `json` option. `headers` is
  applied, but setting `Content-Type` does not create a payload.
- **Redirects are followed**, and `evaluate.status_code` sees the **final**
  response's status, so a 301 or 302 cannot be asserted directly. The limit
  depends on the vantage: `from: host` uses the Go default client and stops at
  10, while the default `from: node` uses `curl --location` and stops at
  curl's own limit (50 unless the node's curl is configured otherwise).
  `timeout` bounds the whole exchange either way, redirects and body read
  included.
- **TLS verification is always on** and cannot be disabled, so `https://` URLs
  with self-signed or expired certificates fail with a request error rather
  than a status-code mismatch. Which trust store decides that also depends on
  the vantage: `from: host` checks against the DART host's root store, while
  `from: node` checks against the node's own CA bundle — so a certificate
  issued by an internal CA can pass from one and fail from the other.
  Inspecting such a certificate is the `tls_cert` test's job — it skips chain
  verification on purpose and exposes `chain_valid`.

A request body, a fixed redirect assertion, or a relaxed TLS check needs an
`execute` test running `curl` on a node:

```yaml
tests:
  - name: API accepts the order payload
    node: app-server
    type: execute
    options:
      command: >
        curl -sS -o /dev/null -w '%{http_code}'
        -X POST -H 'Content-Type: application/json'
        -d '{"id":1}' http://localhost:8080/orders
      evaluate:
        exit_code: 0
        contains: "201"
```

### `file_hash` Details

Note: `file_hash` is the one test type whose `evaluate` block is a closed set.
Only `md5`, `sha1`, and `sha256` are accepted; any other key — including the
standard evaluators such as `exit_code`, `contains`, `match`, `regex`, and
`json_path` — is a config error
(`unknown hash algorithm "..." (supported: md5, sha1, sha256)`). At least one
digest is required, and each value must be a hex string of exactly 32
(`md5`), 40 (`sha1`), or 64 (`sha256`) characters; values are
whitespace-trimmed and compared case-insensitively, so upper-case digests are
fine. Only the checksum tools actually named in `evaluate` are run on the node
(`md5sum`, `sha1sum`, `sha256sum`, joined with `&&` in that order), and each
digest is matched back to its algorithm by length.

### `ping` Details

`ping` accepts `target` or `host` for the destination — one of the two is
required — and `count` defaults to 5. `count` must be a whole number of at
least 1; `0` or a negative value is a config error, not an empty run. With no
`evaluate` block (or an empty one) the implicit check is `packet_loss: 0`, so
any loss fails; supplying any evaluate key replaces that default, meaning a
test that asserts only `contains:` no longer checks loss at all. Evaluate keys
other than `packet_loss` and `rtt_min`/`rtt_avg`/`rtt_max` fall through to the
standard evaluators and apply to the raw `ping` output.

Note: `rtt_min` is a **lower** bound — it passes when the observed minimum RTT
is at least the value — unlike `rtt_avg` and `rtt_max`, which are upper bounds
(they pass when the observed value is at most the bound). Asserting that a
link is *fast* uses `rtt_avg`/`rtt_max`; `rtt_min` asserts a floor, which is
useful for confirming an injected delay is actually present. All three are in
milliseconds and are read from ping's `min/avg/max` summary line, which covers
both the iputils (`rtt min/avg/max/mdev = …`) and busybox
(`round-trip min/avg/max = …`) formats.

The generated command is fixed at `ping -q -c <count> <target>` (the target is
shell-quoted), and `ping` reads no other options — interval, packet size,
deadline, source address or interface, and forcing IPv6 cannot be expressed.
An `execute` test with the full command covers any of those:

```yaml
tests:
  - name: low-rate probe with a deadline
    node: app-server
    type: execute
    options:
      command: ping -q -i 0.5 -w 10 -s 1400 -I eth0 db.internal
      evaluate:
        exit_code: 0
```

### `file_content` Details

`file_content` reads the file by running `cat <filename>` on the node and
applies the `evaluate` keys to stdout. With no `evaluate` block (or an empty
one) the test falls back to a single check named `readable`, which asserts
only that `cat` exited 0 — the content is not inspected. That check name is
what appears in console and JUnit output, and it fails whenever `cat` fails: a
missing file, a permission denial, or a directory path. Binary files are read
as-is, so the match, contains, and regex evaluators see raw bytes.

Note: `ping`, `exists`, `file_content`, `file_hash`, and `service_status` run
commands on the target node (POSIX tools assumed).

### Network Reachability and Certificates

`http_request`, `port_check`, and `tls_cert` probe from the node the test
names. That is what makes their assertions mean what they read as: a test on
`node: app` saying `port_check … host: db.internal, port: 5432` asserts that
*app* reaches the database. The controller sits in a different network
namespace with different routes, resolvers, and firewall rules, so answering
from there describes a different machine.

Each accepts `from: node|host`, defaulting to `node`. `from: host` keeps the
controller's viewpoint, which is the right question for an endpoint that must
be reachable from CI, for a container's published port, and for a minimal
image that ships none of the tools below. Any value other than `node` or
`host` is a configuration error.

Node-side probes are shell based, so they depend on the image:

| Test | Needs on the node | When it is missing |
|---|---|---|
| `http_request` | `curl` | Fails with `curl is required on this node` |
| `tls_cert` | `openssl` | Fails with `openssl is required on this node` |
| `port_check` | bash's `/dev/tcp`, else `nc` with a working `-z` | Reports `unsupported` and fails |

Every one of these fails loudly rather than letting an absent binary read as
an unreachable endpoint. Values reaching a probe — including `{{capture.…}}`
and `{{ var.… }}` substitutions — are shell-quoted after substitution, so a
value carrying a quote is data, never a command.

`port_check` answers firewall and ACL questions: a suite can assert both that
permitted paths work and that blocked ones stay blocked. Its probe prefers
bash's `/dev/tcp` and falls back to `nc` only after proving the node's build
accepts `-z` (busybox builds often don't); a node with no usable method
reports `unsupported` and fails the check rather than guessing.

`tls_cert` fetches the chain from the node with `openssl s_client` and parses
it on the controller, so the same `evaluate` keys apply from either vantage.
Chain verification uses the controller's root store in both modes, which keeps
`chain_valid` from depending on which node happened to fetch the certificate.

`port_check` option semantics:

- `host` and `port` are required. `port` must be between 1 and 65535;
  omitting it fails suite construction with
  `port must be between 1 and 65535`.
- `timeout` is in seconds, defaults to 5, accepts fractional values, and must
  be greater than 0.
- `from` defaults to `node`; any value other than `node` or `host` is a
  configuration error.
- `evaluate.status` defaults to `open`. The observed status is the result's
  stdout, and `closed` also sets a non-zero exit code, so the standard
  evaluators combine with it.
- With `from: node`, the node-side probe is given `timeout <seconds>` (rounded
  up to whole seconds, minimum 1) and DART additionally bounds the command
  suite-side at `timeout + 5` seconds, so a node lacking a working
  `timeout(1)` binary cannot hang the run.

```yaml
tests:
  - name: app server reaches the database
    node: app
    type: port_check
    options:
      host: db.internal
      port: 5432
      from: node          # the default; spelled out here for emphasis

  - name: app server cannot reach admin SSH
    node: app
    type: port_check
    options:
      host: admin.internal
      port: 22
      from: node
      evaluate:
        status: closed    # negative policy assertions

  - name: gateway certificate is not about to expire
    node: local
    type: tls_cert
    options:
      host: vault.internal
      evaluate:
        min_days_remaining: 30
        chain_valid: true
        dns_names: [vault.internal]
```

`tls_cert` option semantics:

- `timeout` is in seconds, accepts fractional values, defaults to 10, and must
  be greater than zero — a zero or negative value is a config error, unlike
  `execute`'s `timeout` where `0` means unbounded. Under `from: host` it bounds
  the TCP dial and the TLS handshake, not the evaluators. Under the default
  `from: node`, `openssl` has no timeout of its own that covers a stalled
  connect, so the only bound is the suite-side command timeout of
  `timeout + 5` seconds.
- `server_name` defaults to `host`. It is sent as the TLS SNI server name
  *and* is used as the hostname the chain verification checks, so a
  `chain_valid: true` assertion fails when the certificate does not cover
  `server_name`. Setting `server_name` is how a virtual host reached by IP or
  through a different DNS name is tested.
- With no `evaluate` block (or an empty one) the implicit check is
  `min_days_remaining: 0`, which passes as long as the certificate has not
  expired.
- `dns_names` requires every listed name to be covered by the certificate.
  Coverage is decided by the same rules a real client applies
  (`x509.VerifyHostname`) against the certificate's DNS SANs and IP SANs, so
  wildcard certificates cover matching subdomains and internal certificates
  carrying only IP SANs match when an IP literal is listed.

Certificate facts are emitted as JSON, so `json_path`, `contains`, and the
other standard evaluators work against them too. Inspection deliberately
skips chain verification during the handshake, so expired or misissued
certificates are still inspectable — assert `chain_valid` explicitly.
`chain_valid` reflects verification of the leaf against the system trust store
using the intermediates the server presented, with `server_name` as the
expected hostname.

The JSON document on the test's stdout has these fields:

| Field | Type | Notes |
|-------|------|-------|
| `subject` | string | Leaf subject as an RFC 2253 DN, e.g. `CN=vault.internal,O=Example,C=US` |
| `issuer` | string | Leaf issuer in the same DN form |
| `dns_names` | array of strings | DNS SANs; `null` when the certificate has none |
| `ip_addresses` | array of strings | IP SANs in text form; `[]` when the certificate has none |
| `not_before` | string | RFC 3339, UTC |
| `not_after` | string | RFC 3339, UTC |
| `days_remaining` | number | Fractional days until `not_after`; negative for an expired certificate |
| `chain_valid` | bool | Verification against the system roots using the presented intermediates and `server_name` |

```yaml
tests:
  - name: gateway certificate comes from the internal CA
    node: local
    type: tls_cert
    options:
      host: vault.internal
      evaluate:
        json_path:
          path: issuer
          equals: "CN=Internal CA,O=Example,C=US"
        chain_valid: true
```

Note: `subject` and `issuer` are whole DN strings, so `json_path` with
`equals` must match the full DN; `issuer_contains` and `subject_contains`
match a substring instead. Array elements are reachable by index
(`path: dns_names[0]`).

### Cluster Consistency

Config drift and quorum questions compare nodes *with each other*, which
per-node tests cannot express. A `consistency` test runs one command
everywhere and compares the results; unlike other types its `node:` list
is not expanded into separate tests.

The compared set defaults to the test's whole `node:` list. `nodes:` is an
optional *narrowing* of that list, not an independent one: every name in
`nodes:` must also appear in the test's `node:` reference, or the run fails at
config load with
`node "x" in nodes: is not listed in the node: reference of test "…"`. If
given, `nodes:` must be a non-empty list. A node may not be listed twice (in
`node:` or in `nodes:`), every name must resolve to a node declared in the
suite's `nodes:` block, and at least two nodes must remain after narrowing — a
consistency test with one node has nothing to compare and is a config error.

```yaml
nodes:
  - name: db-1
    type: local
  - name: db-2
    type: local
  - name: db-3
    type: local

tests:
  - name: compare only the two replicas
    node: [db-1, db-2, db-3]
    type: consistency
    options:
      command: pg_controldata | grep 'Latest checkpoint location'
      nodes: [db-2, db-3]      # subset of node:; db-1 is not compared
```

```yaml
tests:
  - name: every node runs the same config
    node: [web-1, web-2, web-3]
    type: consistency
    options:
      command: sha256sum /etc/app.conf
      # all_equal: true is the default — but only while no evaluate block is given

  - name: exactly one leader is elected
    node: [db-1, db-2, db-3]
    type: consistency
    options:
      command: cluster-role
      evaluate:
        matching:
          pattern: "^leader$"
          count: 1              # the default; 2 leaders (split brain) or 0 both fail

  - name: nodes keep distinct identities
    node: [web-1, web-2]
    type: consistency
    options:
      command: hostname
      evaluate:
        all_equal: false
```

The `matching` map's contract:

- `pattern` is required. It is a Go (RE2) regular expression compiled when the
  config loads, so an invalid pattern is a config error that fails the run
  before any command executes, not a test failure. Matching is
  substring-style (unanchored unless the pattern anchors itself), which is why
  the example writes `"^leader$"`.
- `count` is optional and defaults to `1` — the "exactly one leader" case — so
  the example's `count: 1` can be omitted. It must be a non-negative integer;
  `count: 0` is valid and asserts that no node matches, while negative values,
  non-integers such as `1.5`, and quoted strings such as `"1"` are config
  errors.
- `matching` accepts no keys other than `pattern` and `count`; any other key
  is a config error, so typos are caught rather than ignored.

Failures name which nodes disagree and what each returned
(`web-1,web-2 => "v3" | web-3 => "v2"`). `all_equal` fails in **both**
directions when a node cannot run the command — an outage never satisfies
`all_equal: false` — and comparison is by content digest, so binary outputs
cannot collapse into false agreement.

Comparison covers **stdout only**, right-trimmed of trailing whitespace. Exit
codes and stderr are recorded per node in the JSON report but are not part of
the digest, so nodes that fail the command *identically* (same empty stdout,
same `command not found`) agree and `all_equal: true` passes. Pairing it with
`exit_code: 0` requires the command to have actually succeeded — the
synthesized result carries the highest exit code seen across the nodes, and at
least `1` if any node could not be reached:

```yaml
tests:
  - name: every node runs the same config
    node: [web-1, web-2, web-3]
    type: consistency
    options:
      command: sha256sum /etc/app.conf
      evaluate:
        all_equal: true
        exit_code: 0      # identical failures would otherwise "agree"
```

A node that could not run the command at all (unreachable, timed out) is
distinct from a nonzero exit: it fails `all_equal` in both directions, and its
per-node `exit_code` in the JSON report is `-1`.

Warning: `matching` counts only nodes that successfully ran the command. An
unreachable node is counted as non-matching rather than failing the check, so
`count: 0` passes when the whole cluster is down and `count: 1` passes with
one reachable leader and two dead nodes. Adding `exit_code: 0` alongside
`matching` makes an outage fail the test, since a node that could not run the
command forces the reported exit code to at least 1. (That also fails on a
legitimate non-zero exit from a reachable node, which is usually desirable
here.) Pairing with `all_equal` is rarely the right companion for a `matching`
check — in leader election the nodes are supposed to differ.

`timeout:` bounds each node's command in seconds (fractional values allowed);
it must be non-negative, and `0` or omitted means unbounded. The per-node
report is emitted as JSON so `json_path` and the standard evaluators apply
too.

Note: `{{ fact "self" ... }}` is rejected in a consistency command —
one command runs on many nodes, so "self" has no single meaning; name
the node explicitly.

Note: a `consistency` test's `node:` list is not expanded, but its
non-command hooks are still single-node. `skip_if`, `skip_unless`, and the
test's own `setup:`/`teardown:` commands run on the **first node of the
`node:` list only**; `options.command` runs on every compared node. When
`options.nodes` narrows the comparison to a subset that excludes the first
`node:` entry, those hooks still run on that excluded node — so the first
entry in `node:` should stay a node where the skip conditions and
setup/teardown commands are meaningful, or the condition belongs in a step
instead.

### Built-in Network Facts

LXD and Docker nodes report their own addresses without a fact command:
`{{ fact "web" "ipv4" }}`, `{{ fact "web" "ipv6" }}`, and per-interface or
per-network variants (`ipv4.eth0`, `ipv4.test-net`). User-defined facts of
the same name win, and discovery failures never fail a run.

```yaml
tests:
  - name: load balancer reaches the backend
    node: lb
    type: execute
    options:
      command: curl -sf http://{{ fact "backend" "ipv4" }}:8080/health
      evaluate:
        exit_code: 0
```

**When fact templates are rendered.** Fact templates are only rendered when
the suite actually gathers facts. DART gathers facts when at least one node
declares a `facts:` block, or at least one node is of a type that reports
built-in address facts — `docker`, `lxd`, or `lxd-vm`. The `local`, `ssh`, and
`docker-compose` types do not report built-in facts, so a suite built solely
from those types and carrying no `facts:` block skips template processing
entirely. Rendering happens once per run, after node setup and fact gathering
and before any step or test object is built.

A `{{ fact ... }}` reference that cannot be resolved is an error wherever it
appears, including in a suite that gathers no facts at all:

```text
Error: processing test templates: test "prints an address": ... error calling
fact: no facts are available in this suite, so "db" cannot be resolved: facts
come from a node's facts: block or from the built-in addresses of docker and
lxd nodes
```

Rationale: the literal text used to pass through into the command, so
`command: echo v={{ fact "db" "ipv4" }}` with `evaluate: {exit_code: 0}`
succeeded while asserting nothing about the address.

Note: `--teardown-only` gathers no facts, so a `{{ fact ... }}` reference in a
teardown step fails on that path rather than resolving. Teardown steps meant to
survive a `--teardown-only` run are better written with static values, or with
a command that rediscovers the address on the node.

**Where fact templates are rendered.** `{{ fact "node" "name" }}` is resolved
only in step `options:`, test `options:` (including nested values such as
`evaluate:`), and a test's `setup:`/`teardown:` command lists. It is **not**
resolved in `skip_if`, `skip_unless`, step or test `name:`, or node
`options:` — a fact reference in those places is passed through as literal
text with no error and no warning, so a `skip_if` template silently evaluates
the raw `{{ fact ... }}` string and the test runs instead of skipping.

The three brace syntaxes have different scopes:

- `{{var.name}}` and `{{env.NAME}}` are substituted in the raw YAML before
  parsing, so they work everywhere, including `skip_if`/`skip_unless`.
- `{{capture.name}}` resolves in a test's `command`, `skip_if`, and
  `skip_unless`, and therefore in the option strings of the types that compile
  to a node-side command (see Cross-Test Capture).
- `{{ fact ... }}` resolves only in the option and setup/teardown-command
  scope above.

### Value Extraction and Numeric Assertions

`execute` tests can pull named values out of their output and assert on
them numerically — the building block for performance and regression
gating:

```yaml
tests:
  - name: throughput within baseline
    node: testbed
    type: execute
    options:
      command: loom run --json tcp-100g.yaml
      extract:
        throughput_mbps: { jsonpath: "$.summary.throughput_mbps" }
        p99_us:          { regex: "p99=([0-9.]+)us" }   # capture group 1
      evaluate:
        exit_code: 0
        throughput_mbps: { gte: 11852, within: 12476, tolerance_pct: 5 }
        p99_us:          { lte: 49 }
```

An `evaluate` entry whose name matches an `extract` entry takes a
comparator map instead of a standard evaluation type: `gt`/`gte`/`lt`/`lte`
(`ge`/`le` accepted), `eq`/`ne` (numeric when both sides are numbers,
string otherwise), and `within` with `tolerance_pct` or absolute
`tolerance` for baseline gating. All conditions in the map must hold.

JSON extraction decodes the first JSON value on stdout and ignores everything
after it, so tools that print their JSON first and append log lines afterward
work, as does a stream of concatenated JSON documents — the first one is used.
Leading whitespace is skipped, but any other text *before* the JSON fails the
extraction with `output is not valid JSON: invalid character ...`. Tools that
interleave log lines ahead of their JSON need the logs sent to stderr — only
stdout is parsed — or the output piped through a filter that strips them. The
`jsonpath` extractor and the `json_path` evaluator both accept a leading `$.`
or `$` in the path, so the same path text works in either place; the
`json_path` evaluator is stricter about the document itself, rejecting
trailing content as well as leading content.

Note: `extract:` is an option of the `execute` test type only. Other types
accept the key without complaint and do nothing with it.

### Cross-Test Capture

`execute` tests can record values for later tests — for assertions that
compare across a state transition (before/after a reboot, upgrade, or
rollback):

```yaml
tests:
  - name: record root subvolume id
    node: iso-vm
    type: execute
    options:
      command: btrfs subvolume show / | awk '/Subvolume ID:/{print $3}'
      capture: pre_rollback_root_id     # stores trimmed stdout

  - name: rollback recreated the root subvolume
    node: iso-vm
    type: execute
    options:
      command: |
        [ "$(btrfs subvolume show / | awk '/Subvolume ID:/{print $3}')" -gt "{{capture.pre_rollback_root_id}}" ]
      evaluate:
        exit_code: 0
```

`capture:` takes a bare name (whole trimmed stdout) or a map of names to
`{jsonpath}`/`{regex}` extractors. Capture names must match
`^[A-Za-z_][A-Za-z0-9_]*$` — ASCII letters, digits, and underscores, with no
leading digit and no hyphens or dots. That applies to both the bare-name form
and every key of the map form, and it is checked at config load, so
`capture: pre-rollback-id` fails the suite before any test runs with
`capture name "pre-rollback-id" in test "..." must match
^[A-Za-z_][A-Za-z0-9_]*$`. The `{{capture.name}}` reference syntax accepts the
same character set.

Later tests reference values as `{{capture.name}}` in `command`, `skip_if`,
and `skip_unless`; a reference to a value nothing captured fails the test
rather than running a mangled command. Capture references inside an
`evaluate` block are **not** interpolated — evaluators are built once at
config load from the raw YAML values, so a `{{capture.x}}` written there is
compared as a literal string and the check fails on every run with no config
error to flag it. Comparing against a captured value belongs inside the
`command` itself, as in the example above. See the
[Test Evaluation Reference](evaluation.md) for the full interpolation scope.

The capture store is per-run: every `-i` iteration starts empty, so the
capturing test must run earlier in the same iteration and must not be skipped
or excluded by `--only`/`--skip`. Capturing the same name twice keeps the most
recent value.

Note: `capture:` and `extract:` are options of the `execute` test type only.
Every other test type (`exists`, `file_content`, `file_hash`, `http_request`,
`ping`, `port_check`, `service_status`, `reboot`, `tls_cert`, `consistency`)
accepts the keys without error and silently records nothing, and because
unrecognized keys inside `options:` are ignored rather than rejected,
`--check` still reports the suite valid. The mistake surfaces only later, when
the referencing test fails with `no captured value named <name>` — an error
that points at the wrong test. Capturing from an HTTP body, a file, or a TLS
endpoint therefore means re-expressing the *capturing* test as an `execute`
test (`curl -s …`, `cat …`, `openssl s_client …`) with `capture:`.

Referencing a captured value is wider than defining one. `{{capture.name}}`
resolves in `skip_if` and `skip_unless` on every test type, in the command of
the command-backed types (`execute`, `exists`, `file_content`, `file_hash`,
`ping`, `service_status`) — including option strings that become part of that
command, such as `path: /data/{{capture.dir}}/log` — in `consistency`'s
command, and in the probe values of the network types running on the node:
`http_request`'s `url` and `headers`, `port_check`'s `host`, and `tls_cert`'s
`host` and `server_name`. It does not resolve for the types that act from the
DART host — the same three with `from: host`, and `reboot` — where the literal
`{{capture.name}}` text is used as-is.

Note: a test with a `node:` list is expanded into one test per node, and every
copy shares the same `options` map — including the same `capture:` name. All
copies write into the one suite-wide store keyed by that name, so each node
overwrites the previous one and only the value from the **last node in the
list** survives for `{{capture.name}}` references. Tests run sequentially in
expansion order, so this is deterministic rather than a race. Keeping a value
per node means giving each node its own single-node test with a distinct
capture name:

```yaml
tests:
  - name: record boot id on web-1
    node: web-1
    type: execute
    options:
      command: cat /proc/sys/kernel/random/boot_id
      capture: boot_web1

  - name: record boot id on web-2
    node: web-2
    type: execute
    options:
      command: cat /proc/sys/kernel/random/boot_id
      capture: boot_web2
```

### Per-Test Setup and Teardown

Any test accepts `setup:` and `teardown:` — plain lists of shell command
strings, not step objects like the suite-level `setup:`/`teardown:` blocks:

```yaml
nodes:
  - name: app
    type: local

tests:
  - name: service answers after config swap
    node: app
    type: execute
    setup:
      - cp /etc/app.conf /tmp/app.conf.bak
      - install -m0644 /tmp/test.conf /etc/app.conf
    teardown:
      - mv /tmp/app.conf.bak /etc/app.conf
    options:
      command: curl -sf http://localhost:8080/health
      evaluate:
        exit_code: 0
```

- Both lists run on the test's own node, in order. For a multi-node
  `consistency` test they run only on the first node in `node:`, not on every
  peer.
- They are available on every test type.
- `setup` runs once before the first attempt and `teardown` once after the
  last; both sit outside the retry loop, so `retry:` reruns only the test
  command and its evaluations. `teardown` still runs when the test fails or
  errors, since it is cleanup.
- A test skipped by `skip_if`/`skip_unless` runs neither list — the skip
  condition is evaluated before the test.
- Failure means the command could not be run at all (transport error, timeout,
  container or session error). A command that runs and exits nonzero is not
  treated as a failure — its exit code is discarded. Gating on an exit code
  therefore needs `skip_if`/`skip_unless` or a real test, not a setup command.
- A `setup` command failure reports the test as an error, the test body never
  runs, and the run aborts (node and platform teardown still happen). A
  `teardown` command failure is surfaced after the test's evaluations are
  reported, so a passing test still shows its results, but the run then aborts
  because the system state is unknown. Exception: when the test itself failed
  with a command timeout, the timeout is reported as the failure and the
  teardown error is dropped.
- Node facts and `{{var.*}}`/`{{env.*}}` references are rendered into these
  commands. Capture references (`{{capture.*}}`) are **not** interpolated
  here — unlike in `skip_if`/`skip_unless` and test options — so captures
  cannot be used in per-test setup and teardown commands.

### Suite Variables and Tags

Suites parameterize with a `vars:` block, `{{var.name}}` / `{{env.NAME}}`
references (resolved at config load — a numeric var in a numeric position
stays a number), and `--vars` CLI overrides (long form `--vars`, short form
`-var`). Capture references and fact templates are untouched. Unresolved
references are config errors.

```yaml
suite: API smoke
vars:
  target: 10.0.0.5      # override per run: dart -c suite.yaml --vars target=192.168.1.1
nodes:
  - name: local
    type: local
tests:
  - name: api answers
    node: local
    type: http_request
    tags: [smoke, network]
    options:
      url: "http://{{var.target}}:8080/health"
```

Five rules govern variable resolution:

**Variable names.** A reference is recognised only when the name matches
`[A-Za-z_][A-Za-z0-9_]*` — letters, digits, and underscores, not starting with
a digit. Names containing hyphens or dots are not recognised at all:
`{{var.my-target}}` is left in the file verbatim, with no substitution and
**no config error**, so the literal text reaches the command and the test
fails on a mangled invocation. This applies to `{{env.NAME}}` equally, and
`--vars my-target=x` is accepted by the CLI without complaint yet still never
substitutes. Use `my_target`. A well-formed but undefined name *is* an error:
`unresolved references: var.nope (define in the vars block, pass --vars, or set
the environment variable)`. Whitespace inside the braces is allowed, so
`{{ var.x }}` works.

**Quote references whose value contains YAML-significant characters.**
Substitution happens in the raw YAML before parsing, so an unquoted `#` would
truncate the line and a `:` would corrupt the mapping. If a value contains any
of ``# : " ' { } [ ] & * ! | > % @ ` ,`` the reference must sit inside quotes
on its line, or load fails with
`var.X value "…" contains YAML-significant characters; quote the reference
(e.g. "{{var.X}}")`. Writing `"{{var.x}}"` unconditionally, as the example
above does, avoids the problem.

**Vars may be defined in terms of other vars and env.** Var values are
resolved against other vars and `{{env.NAME}}` before substitution into the
document, so `b: "{{var.a}}2"` works. Resolution is bounded at 10 rounds;
definitions that are too deeply nested or mutually circular fail with
`var definitions reference each other too deeply or circularly`, and a
self-referential definition fails with
`var "a" could not be fully resolved (circular or self-referential definition)`.
Unresolved references *inside* var values are reported separately as
`unresolved references in var values: …`.

**References inside comments are inert.** A `{{var.x}}` appearing after an
unquoted `#` on a line is skipped entirely — never substituted, and an
undefined name there is not an error. Documentation and commented-out examples
are therefore safe.

**Values must be single-line.** A var value containing a newline — whether
from the `vars:` block or `--vars` — is rejected with
`variable values must be single-line`, because substitution is inline and a
newline would rewrite the document's structure.

`--vars` takes comma-separated `key=value` pairs:
`dart -c suite.yaml --vars target=192.168.1.1,port=8443`. A CLI pair replaces
the same-named entry in the suite's `vars:` block; entries the CLI does not
name keep their suite values. Note: the flag value is split on every comma
before the `key=value` cut, so a variable value may not itself contain a
comma — such values belong in the `vars:` block, which is not split, or in an
environment variable referenced as `{{env.NAME}}`.

Tests carry `tags:`; run subsets with `--only tag=network` (any listed tag
matches) and exclude with `--skip tag=slow`. Steps are never filtered, so
setup/teardown chains stay intact; the run reports how many tests the filter
excluded. A filter that excludes *every* test is an error — the run stops with
`the --only/--skip tag filter excluded every test; nothing ran (check the tag
names against the suite)` and exits non-zero, so a mistyped tag name cannot
masquerade as a green run.

### Timeouts and Retries

Any `execute` test or step accepts `timeout:` (seconds; `0`/omitted means
unbounded) — a hung command fails the test with a clear `timeout` check
failure instead of hanging the suite, and teardown still runs. Note: the
remote process may keep running; the timeout bounds the suite's wait, and
a retried timeout re-awaits the same in-flight command rather than
launching another.

`wait_for` is bounded by its own clock rather than by the shared `timeout:`
option: `command` is its only required option, `timeout` defaults to 60
seconds, and `interval` defaults to 2 seconds. Both must be numbers, and a
value supplied explicitly must be positive — an omitted value falls back to
the default without error. Each poll is bounded by one `interval` against a
single in-flight invocation, so a check slower than the interval is re-awaited
rather than relaunched (invocations never overlap) and the step may overrun
its deadline by at most one interval.

`retry` is rejected on `reboot` tests at config load — retrying one would
power-cycle the target on every failed evaluation, so the suite never starts.

Eventually-consistent assertions retry until they pass or time out:

```yaml
tests:
  - name: cluster elects a leader
    node: n1
    type: execute
    retry:
      timeout: 60      # required, > 0; keep retrying up to 60s
      interval: 5      # between attempts (default 2, must be >= 0 and < timeout)
    options:
      command: cluster-status --leader
      timeout: 10      # per-attempt command bound
      evaluate:
        exit_code: 0
        regex: "leader=node[0-9]+"
```

Both `retry` fields are in seconds and accept fractional values. `timeout` is
required whenever a `retry:` block is present and must be greater than zero —
an omitted or zero `timeout` is a config error, not "no retry"; omitting the
whole `retry:` block is how retry is disabled. `interval` may not be negative;
omitting it (or setting it to `0`) uses the default of 2 seconds. The interval
must be strictly smaller than the timeout, or the retry loop could never take
a second attempt, so the config is rejected at load time with
`retry.interval (2s) must be smaller than retry.timeout (1s) in test "…" or
retry can never engage`. The comparison uses the *defaulted* interval, so
`retry: { timeout: 2 }` is an error while
`retry: { timeout: 2, interval: 0.5 }` is valid. All three rules are reported
with the test name and source location before any test runs.

`retry` reruns the test command and its evaluations; per-test `setup:` and
`teardown:` commands run once, outside the loop. The matching `wait_for` step
covers setup:

```yaml
setup:
  - name: wait for the API to answer
    node: app
    step:
      type: wait_for
      options:
        command: curl -sf http://localhost:8080/health
        timeout: 120   # seconds, default 60
        interval: 3    # seconds between polls, default 2
```

### Conditional Skips

Any test can declare a skip condition — a command run on the test's node
before the test executes:

```yaml
tests:
  - name: RKE2 server is running
    node: iso-vm
    type: service_status
    skip_unless: which aether-ops-bootstrap   # nonzero exit → skip
    options:
      service: rke2-server

  - name: legacy migration check
    node: iso-vm
    type: execute
    skip_if: test -f /etc/new-style-config    # zero exit → skip
    options:
      command: legacy-migrate --verify
      evaluate:
        exit_code: 0
```

Skipped tests render with a distinct yellow `skipped` status and are
counted separately in the results (`Pass / Fail / Skip`), so a suite that
quietly skips assertions can never read as fully green. Skips do not
affect the exit code. An error running the condition command itself fails
the run — a broken condition never silently passes or skips.

The console shows only the yellow `skipped` status; `-v` also prints the
condition that fired (`skip_if condition met: <command>` or
`skip_unless condition not met: <command>`, with captures already
interpolated). The same reason is written to any report requested with `-r` —
the `reason` field in JSON, the `message` attribute of `<skipped>` in JUnit —
so CI runs keep it without needing verbose output.

Note: skip conditions resolve `{{capture.name}}` and `{{var.name}}` but not
`{{ fact ... }}`. Fact rendering covers a test's `options` (including nested
maps and lists), its per-test `setup` and `teardown` commands, and a step's
`options`; a skip condition is passed to the node verbatim, so a fact
reference reaches the shell as literal `{{ fact "web" "ipv4" }}` text. Gating
on a fact-derived value means putting the fact reference in the test's `setup`
command or in `options` instead. See Built-in Network Facts for the full
templating scopes.

For a `consistency` test, the skip condition runs on the first node of the
`node:` list only, even though the test's command runs on every compared node.

---
