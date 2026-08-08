# DART

**Test the things unit tests can't reach.**

Unit tests prove your functions work. DART proves your *system* works —
that the service actually starts on a clean machine, that the config you
ship parses where it lands, that the firewall rule really blocks the
port, that the cluster still agrees after a node reboots.

You describe the environment and the assertions in YAML; DART builds the
environment (containers, VMs, or remote hosts), runs the checks, tears
everything down, and returns a CI-friendly exit code with a JUnit report.

> **Note:** DART is in active development. Behaviour and configuration
> may change between releases.

```yaml
suite: my service works
nodes:
  - name: box
    type: docker
    options: { image: ubuntu:24.04 }
tests:
  - name: the binary runs
    node: box
    type: execute
    options:
      command: /usr/local/bin/myservice --version
      evaluate:
        exit_code: 0
        regex: "v[0-9]+\\.[0-9]+"
```

```console
$ dart -c suite.yaml
[+] Running tests
  00001: [ box ] the binary runs ... passed

[+] Results
  Pass: 00001
  Fail: 00000
  Time: 1.2s
```

---

## Install

```bash
# Recommended: install script (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/bgrewell/dart/main/install.sh | bash

# Or with Go
go install github.com/bgrewell/dart/cmd/dart@latest

# Or build from source
git clone https://github.com/bgrewell/dart && cd dart && make build
```

Verify it works, then validate a suite without running anything:

```bash
dart --version
dart -c suite.yaml --check     # parses + validates, touches no infrastructure
```

DART needs nothing else to test the local machine or remote hosts over
SSH. For container and VM nodes, install Docker or LXD/Incus — DART talks
to whichever is present.

---

## Your first suite

Create `suite.yaml`:

```yaml
suite: hello dart
nodes:
  - name: local          # the machine running DART
    type: local
tests:
  - name: the tools I need are installed
    node: local
    type: execute
    options:
      command: git --version
      evaluate:
        exit_code: 0
        contains: "git version"
```

Run it with `dart -c suite.yaml`. That's the whole loop: **nodes** define
where things run, **tests** define what must be true, and `evaluate`
defines what "true" means.

---

## Common things developers test

### Does my service actually start and serve?

Build the image, start it, wait for readiness, then assert on real
behaviour — not a mock.

```yaml
suite: service smoke test
nodes:
  - name: app
    type: docker
    options:
      image: myservice:latest
      ports: ["8080:8080"]
setup:
  - name: wait for the API to answer
    node: app
    step:
      type: wait_for
      options:
        command: curl -sf http://localhost:8080/health
        timeout: 60
tests:
  - name: health endpoint reports healthy
    node: app
    type: http_request
    options:
      url: http://localhost:8080/health
      evaluate:
        status_code: 200
        json_path: { path: status, equals: healthy }

  - name: service survives a restart
    node: app
    type: execute
    options:
      command: kill -HUP 1 && sleep 2 && curl -sf http://localhost:8080/health
      evaluate:
        exit_code: 0
```

### Does my config deploy correctly?

Render a template, push it to the node, and verify what actually landed.

```yaml
setup:
  - name: render the config for this node
    node: app
    step:
      type: file_template
      options:
        source: fixtures/app.conf.tmpl     # Go template: {{ .port }}
        dest: /etc/myapp/app.conf
        overwrite: true
        mode: "0640"
        values: { port: 8080, workers: 4 }
tests:
  - name: the deployed config is what we intended
    node: app
    type: file_content
    options:
      filename: /etc/myapp/app.conf
      evaluate:
        contains: "port = 8080"

  - name: the service accepts it
    node: app
    type: execute
    options:
      command: myservice --config /etc/myapp/app.conf --validate
      evaluate:
        exit_code: 0
```

### Is the package installable on a clean machine?

The classic "works on my machine" bug: a dependency you have installed
and your users don't.

```yaml
suite: clean install
nodes:
  - name: clean
    type: docker
    options: { image: ubuntu:24.04 }
setup:
  - name: copy the built package in
    node: clean
    step:
      type: file_push
      options: { source: dist/myservice.deb, dest: /tmp/myservice.deb }
  - name: install it
    node: clean
    step:
      type: execute
      options: { command: "apt-get install -y /tmp/myservice.deb" }
tests:
  - name: the service starts on a machine that has nothing else
    node: clean
    type: service_status
    options: { service: myservice }
```

Run the same suite against several images at once by listing nodes:
`node: [ubuntu, debian, alpine]` expands into one test per node.

---

## What unit tests can't do

This is where DART earns its place in CI. Every example below asserts
something a unit test — or even a normal integration test — structurally
cannot.

### Survive a real reboot

Mocks can't power-cycle a machine. Capture state, reboot for real, and
prove the service came back.

```yaml
tests:
  - name: record the boot id
    node: vm
    type: execute
    options:
      command: cat /proc/sys/kernel/random/boot_id
      capture: boot_before

  - name: reboot the machine
    node: vm
    type: reboot
    options:
      mode: graceful        # 'force' models a power cut
      timeout: 300

  - name: the machine really rebooted
    node: vm
    type: execute
    options:
      command: '[ "$(cat /proc/sys/kernel/random/boot_id)" != "{{capture.boot_before}}" ]'
      evaluate: { exit_code: 0 }

  - name: the service came back by itself
    node: vm
    type: service_status
    options: { service: myservice }
```

### Wait for a distributed system to converge

Elections settle, DNS propagates, replicas catch up. `retry` reruns the
test until reality agrees or the deadline passes — no `sleep 30` guesses.

```yaml
  - name: the cluster elects exactly one leader
    node: [db-1, db-2, db-3]
    type: consistency
    retry: { timeout: 90, interval: 5 }
    options:
      command: cluster-role
      evaluate:
        matching: { pattern: "^leader$", count: 1 }   # split brain fails
```

### Prove firewall and network policy

Reachability is a property of *where you stand*. `from: node` probes from
the machine that matters, and asserting `closed` proves a rule works.

```yaml
  - name: the app server can reach the database
    node: app
    type: port_check
    options: { host: db.internal, port: 5432, from: node }

  - name: the app server cannot reach admin SSH
    node: app
    type: port_check
    options:
      host: admin.internal
      port: 22
      from: node
      evaluate: { status: closed }
```

### Catch configuration drift across a fleet

Per-node tests can't compare nodes to *each other*.

```yaml
  - name: every node runs identical config
    node: [web-1, web-2, web-3]
    type: consistency
    options:
      command: sha256sum /etc/app.conf
```

A failure names who disagrees:
`web-1,web-2 => "3f2a…" | web-3 => "9c1b…"`.

### Gate on performance regressions

Extract measured numbers and assert on them with tolerances.

```yaml
  - name: throughput has not regressed
    node: testbed
    type: execute
    options:
      command: loom run --json tcp-100g.yaml
      extract:
        throughput_mbps: { jsonpath: "$.summary.throughput_mbps" }
        p99_us:          { regex: "p99=([0-9.]+)us" }
      evaluate:
        throughput_mbps: { within: 12476, tolerance_pct: 5 }
        p99_us:          { lte: 49 }
```

### Test destructively, then roll back

Snapshot before you break things; restore in teardown. Far faster than
rebuilding the environment.

```yaml
setup:
  - name: capture clean state
    node: vm
    step:
      type: snapshot
      options: { name: clean }
teardown:
  - name: roll back
    node: vm
    step:
      type: snapshot
      options: { name: clean, action: restore }
```

### Know your certificates before your users do

```yaml
  - name: the gateway certificate is not about to expire
    node: local
    type: tls_cert
    options:
      host: gateway.internal
      evaluate:
        min_days_remaining: 30
        chain_valid: true
```

---

## Running in CI

DART exits non-zero when tests fail and writes reports CI can render:

```bash
dart -c suite.yaml \
  --report junit:results.xml,json:results.json \
  --log run.log
```

```yaml
# .github/workflows/integration.yml
- name: Integration tests
  run: dart -c tests/integration.yaml --report junit:results.xml
- name: Publish results
  uses: dorny/test-reporter@v1
  if: always()
  with: { path: results.xml, reporter: java-junit }
```

Useful flags for pipelines:

| Flag | Why |
|---|---|
| `--check` | Validate the suite without touching infrastructure — a fast pre-commit lint |
| `--report junit:PATH` | Per-test results for GitHub/GitLab/Jenkins test panels |
| `--vars key=value` | Point one suite at staging or prod without editing YAML |
| `--only tag=smoke` | Run a subset; `--skip tag=slow` excludes |
| `-s`, `--stop-on-error` | Stop at the first failure |
| `-d`, `--debug` | Stream command output live while debugging a suite |

Parameterize suites so one file serves every environment:

```yaml
vars:
  target: staging.internal      # dart -c suite.yaml --vars target=prod.internal
tests:
  - name: api answers
    node: local
    type: http_request
    tags: [smoke]
    options: { url: "https://{{var.target}}/health" }
```

---

## Documentation

The [full documentation](docs/) covers everything in detail:

- **[Node types](docs/node-types.md)** — local, Docker, Docker Compose,
  LXD/Incus, SSH; remote daemons, ISO boot, security defaults
- **[Test types](docs/tests.md)** — every test type plus retries,
  timeouts, skips, captures, variables, and tags
- **[Evaluation reference](docs/evaluation.md)** — every `evaluate`
  check available
- **[Steps](docs/steps.md)** — setup and teardown: commands, packages,
  files, templates, snapshots, waits
- **[Command line](docs/cli.md)** — all flags and exit codes

Runnable examples live in [`examples/`](examples/).

---

## How it works

A suite runs in phases, and cleanup always happens:

1. **Platform setup** — create Docker networks/images, LXD projects/profiles
2. **Node setup** — start containers, connect to hosts
3. **Facts** — gather node facts (addresses are built in)
4. **Setup steps** — install, configure, wait for readiness
5. **Tests** — run and evaluate
6. **Teardown steps**, then **node** and **platform teardown**

Nodes are torn down even when a run fails, and reports are written even
when it aborts early — a crashed run still tells CI what happened.

---

## Contributing

Issues and pull requests are welcome. `go test ./...` should pass and
`gofmt -l .` should be empty before you open one.

## License

Distributed under the license in [LICENSE](LICENSE).
