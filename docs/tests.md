# Test Types

Tests run against a node and evaluate the outcome. This is the full reference for every test type and the behaviours shared across them (retries, skips, captures, variables).

[← Back to the README](../README.md)

Tests run against a node and evaluate the outcome. `execute` is the
general-purpose type; the others are shorthand for common checks and accept
the same `evaluate` keys where noted.

| Type | What it does | Key options |
|------|--------------|-------------|
| `execute` | Run a command on the node, evaluate its result | `command`, `evaluate` (full reference below) |
| `exists` | Check a path exists on the node (`test -e`) | `path`; `evaluate.exists: true\|false` |
| `file_content` | Read a file on the node, evaluate its content | `filename`; standard `evaluate` keys apply to the content |
| `file_hash` | Verify file checksums on the node | `filename`; `evaluate.md5/sha1/sha256` hex digests |
| `service_status` | Check a systemd unit state on the node | `service`; `evaluate.status` (default `active`) |
| `ping` | Ping a target from the node | `target`, `count`; `evaluate.packet_loss` (max %), `rtt_min/rtt_avg/rtt_max` (ms) |
| `http_request` | HTTP request from the DART host | `url`, `method`, `headers`, `timeout`; `evaluate.status_code` plus standard keys against the body |
| `port_check` | TCP connect from the DART host | `host`, `port`, `timeout`; `evaluate.status: open\|closed` |
| `reboot` | Restart the node mid-suite and wait until it accepts commands | `mode: graceful\|force`, `ready_command`, `timeout` (lxd and ssh nodes) |
| `consistency` | Compare one command's output **across** nodes | `command`, `nodes`; `evaluate.all_equal`, `matching: {pattern, count}` |
| `tls_cert` | Inspect a TLS endpoint's certificate | `host`, `port` (443), `server_name`, `timeout`; `evaluate.min_days_remaining`, `dns_names`, `issuer_contains`, `subject_contains`, `chain_valid` |

Any test also accepts test-level `retry:` (see Timeouts and Retries) and
`skip_if`/`skip_unless` (see Conditional Skips).

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

`reboot` is also available as a setup/teardown step with the same
options. On LXD nodes the readiness wait reuses the node's `boot_wait`
configuration; `mode: force` kills the instance without a clean shutdown,
which is what crash-safety suites need. On SSH nodes the reboot is issued
over the session (passwordless sudo or root) and DART reconnects until the
host answers again.

```yaml
tests:
  - name: API serves healthy status
    node: app-server
    type: http_request
    options:
      url: http://localhost:8080/health
      evaluate:
        status_code: 200
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

Note: `ping`, `exists`, `file_content`, `file_hash`, and `service_status`
run commands on the target node (POSIX tools assumed); `http_request`,
`tls_cert`, and `port_check` act from the host running DART unless told
otherwise.

### Network Reachability and Certificates

`port_check` answers firewall and ACL questions when pointed at a node:
`from: node` runs the probe on the node's own shell, so a suite can assert
both that permitted paths work and that blocked ones stay blocked. The
probe prefers bash's `/dev/tcp` and falls back to `nc` only after proving
the node's build accepts `-z` (busybox builds often don't); a node with no
usable method reports `unsupported` and fails the check rather than
guessing. Host values are passed as arguments, never interpolated into a
shell string.

```yaml
tests:
  - name: app server reaches the database
    node: app
    type: port_check
    options:
      host: db.internal
      port: 5432
      from: node          # probe from the node, not the DART host

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

Certificate facts are emitted as JSON, so `json_path`, `contains`, and the
other standard evaluators work against them too. Inspection deliberately
skips chain verification during the handshake, so expired or misissued
certificates are still inspectable — assert `chain_valid` explicitly.

### Cluster Consistency

Config drift and quorum questions compare nodes *with each other*, which
per-node tests cannot express. A `consistency` test runs one command
everywhere and compares the results; unlike other types its `node:` list
is not expanded into separate tests.

```yaml
tests:
  - name: every node runs the same config
    node: [web-1, web-2, web-3]
    type: consistency
    options:
      command: sha256sum /etc/app.conf
      # all_equal: true is the default check

  - name: exactly one leader is elected
    node: [db-1, db-2, db-3]
    type: consistency
    options:
      command: cluster-role
      evaluate:
        matching:
          pattern: "^leader$"
          count: 1              # 2 leaders (split brain) or 0 both fail

  - name: nodes keep distinct identities
    node: [web-1, web-2]
    type: consistency
    options:
      command: hostname
      evaluate:
        all_equal: false
```

Failures name which nodes disagree and what each returned
(`web-1,web-2 => "v3" | web-3 => "v2"`). A node that cannot run the
command fails the comparison in **both** directions — an outage never
satisfies `all_equal: false` — and comparison is by content digest, so
binary outputs cannot collapse into false agreement. `timeout:` bounds
each node's command, and the per-node report is emitted as JSON so
`json_path` and the standard evaluators apply too.

Note: `{{ fact "self" ... }}` is rejected in a consistency command —
one command runs on many nodes, so "self" has no single meaning; name
the node explicitly.

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
JSON extraction reads the first JSON document in the output, so tools
that mix JSON with log lines work.

### Cross-Test Capture

Tests can record values for later tests — for assertions that compare
across a state transition (before/after a reboot, upgrade, or rollback):

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
`{jsonpath}`/`{regex}` extractors. Later tests reference values as
`{{capture.name}}` in `command`, `skip_if`, and `skip_unless`; a
reference to a value nothing captured fails the test rather than running
a mangled command. Values persist across `-i` iterations and are
overwritten by each run.

### Suite Variables and Tags

Suites parameterize with a `vars:` block, `{{var.name}}` / `{{env.NAME}}`
references (resolved at config load — a numeric var in a numeric position
stays a number), and `--vars key=value[,key=value]` CLI overrides. Capture
references and fact templates are untouched. Unresolved references are
config errors.

```yaml
suite: API smoke
vars:
  target: 10.0.0.5      # override per run: dart -c suite.yaml --vars target=192.168.1.1
tests:
  - name: api answers
    node: local
    type: http_request
    tags: [smoke, network]
    options:
      url: http://{{var.target}}:8080/health
```

Tests carry `tags:`; run subsets with `--only tag=network` (any listed tag
matches) and exclude with `--skip tag=slow`. Steps are never filtered, so
setup/teardown chains stay intact; the run reports how many tests the
filter excluded.

### Timeouts and Retries

Any `execute` test or step accepts `timeout:` (seconds; `0`/omitted means
unbounded) — a hung command fails the test with a clear `timeout` check
failure instead of hanging the suite, and teardown still runs. Note: the
remote process may keep running; the timeout bounds the suite's wait, and
a retried timeout re-awaits the same in-flight command rather than
launching another. `wait_for` differs: its `timeout` is required and must
be positive, since the step's whole purpose is to give up eventually.
`retry` is rejected on `reboot` tests — retrying one would power-cycle
the target on every failed evaluation.

Eventually-consistent assertions retry until they pass or time out:

```yaml
tests:
  - name: cluster elects a leader
    node: n1
    type: execute
    retry:
      timeout: 60      # keep retrying up to 60s
      interval: 5      # between attempts (default 2)
    options:
      command: cluster-status --leader
      timeout: 10      # per-attempt command bound
      evaluate:
        exit_code: 0
        regex: "leader=node[0-9]+"
```

`retry` reruns the test command and its evaluations (setup/teardown
commands run once); it works on every test type. The matching `wait_for`
step covers setup:

```yaml
setup:
  - name: wait for the API to answer
    node: app
    step:
      type: wait_for
      options:
        command: curl -sf http://localhost:8080/health
        timeout: 120
        interval: 3
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

---
