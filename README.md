# DART

**Test the things unit tests can't reach.**

Unit tests prove your functions work. DART proves your *system* works —
that the service actually starts on a clean machine, that the config you
ship parses where it lands, that the firewall rule really blocks the
port, that the cluster still agrees after a node reboots.

You describe the environment and the assertions in YAML; DART builds the
environment (containers, VMs, or remote hosts), runs the checks, tears
everything down, and returns a CI-friendly exit code with a JUnit report.

📖 **[Documentation](https://bgrewell.github.io/dart/)** — guides, every
test and step type, and the full evaluation reference.

> **Note:** DART is in active development. Behaviour and configuration
> may change between releases.

```yaml
suite: my service works
nodes:
  - name: box
    type: docker
    options: { image: myservice:latest }
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
[+] Running test setup
  [ box ] running setup ... done
[+] Running tests
  00001: [ box ] the binary runs ... passed

[+] Running test teardown
  [ box ] running teardown ... done

[+] Results
  Pass: 00001
  Fail: 00000
  Time: 1.2s
```

---

## Install

```bash
# Recommended: install script (Linux x86_64 / arm64)
curl -fsSL https://raw.githubusercontent.com/bgrewell/dart/main/install.sh | bash

# Any platform Go supports (macOS, Windows, other architectures)
go install github.com/bgrewell/dart/cmd/dart@latest

# Or build from source (produces a Linux binary; the Makefile pins GOOS=linux)
git clone https://github.com/bgrewell/dart && cd dart && make build
```

The install script supports Linux on x86_64 and arm64 only — those are the
only binaries the release workflow publishes — and it verifies the
downloaded binary against the release `checksums.txt` before installing,
which is the main reason to prefer it over `go install`. On macOS, Windows,
or any other architecture, use `go install`. Note: `make build` sets
`GOOS=linux` and always produces a Linux binary regardless of the host.

The script installs to `/usr/local/bin` by default; `DART_INSTALL_DIR`
selects another location and `DART_VERSION` pins a specific release tag
instead of the latest. Installing into a directory the current user cannot
write invokes `sudo` for the final move.

The `go install` and build-from-source paths require Go 1.26.5 or newer,
the minimum set in `go.mod`. Go 1.21 and later fetch that toolchain
automatically unless `GOTOOLCHAIN=local` is set; older releases fail with a
version error. The install script and the released binaries need no Go
toolchain.

Verify it works, then validate a suite without running anything:

```bash
dart --version                 # short form: -V
dart -c suite.yaml --check     # parses + validates, touches no infrastructure
```

Note: `version`, build date, revision and branch are injected at link time
by `make build` and the release workflow. A binary produced by plain
`go install` reports `dev` for all four.

DART needs nothing else to test the local machine or remote hosts over
SSH. For container and VM nodes, install Docker or LXD/Incus — DART talks
to whichever is present.

---

## Your first suite

Create `suite.yaml`:

```yaml
suite: hello dart
nodes:
  - name: local          # any name; tests refer to the node by it
    type: local          # type: local is what makes this the machine running DART
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

Note: node names are free-form and must be unique within a suite; `local`
here is a name this example chose, not a reserved one. A test that names a
node the suite does not declare aborts the run before any setup step or test
executes — though only after the nodes themselves have been created, since the
check happens when tests are constructed. `dart -c suite.yaml --check` catches
the same typo without touching infrastructure. The `local` *type*, on the other
hand, is limited to one node per suite.

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

Note: the network checks — the `http_request`, `port_check`, and `tls_cert`
tests, and the `http_request` and `dns_request` steps — probe from the node
they name. That is what makes them mean what they read as: the test above
reaches `localhost` from inside the container, the same namespace the
`wait_for` step's `curl` runs in. A `from` option chooses the vantage point,
and `from: host` asks the same question from the machine running DART —
which is what the published `ports: ["8080:8080"]` makes possible, and what
you want for an endpoint that must be reachable from CI.

The node-side probes are shell based and depend on tools being present in
the image: `http_request` requires `curl`, `tls_cert` requires `openssl`,
`dns_request` uses whichever of `getent`, `dig`, `host`, or `nslookup` it
finds, and `port_check` prefers bash's `/dev/tcp` and falls back to `nc`.
Each fails loudly when its tool is absent rather than reading as an
unreachable endpoint. A minimal image that ships none of them is a case for
`from: host`.

Warning: a docker node's image must run a long-lived foreground process.
DART creates the container from the image's own `CMD`/`ENTRYPOINT` with no
TTY and no attached stdin, so an image whose default command is an
interactive shell — `ubuntu`, `debian`, `alpine` — exits the moment it
starts. Node setup then polls for up to two minutes waiting for the
container to report running and fails with
`timeout waiting for container ... to become ready`. Give such an image a
`command:` that stays up (`command: ["sleep", "infinity"]`), use a
purpose-built service image, or put it on an `lxd` (or `lxd-vm`) or `ssh`
node instead.

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
        create_dir: true                   # mkdir -p the parent directory
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

Both `create_dir` and `overwrite` default to false: without them the step
fails when the parent directory is missing, and fails again when the
destination already exists.

Note: every local path a suite writes follows one rule — absolute paths are
used as-is, `~` is the invoking user's home directory, and anything else is
relative to the directory holding the suite file. That covers file-step
sources and destinations, docker `volumes`, LXD disk `source`s, SSH keys and
`known_hosts`, LXD certificates, `compose_file`, `docker.images[].dockerfile`,
and `!!load_from`. A suite is therefore portable: it behaves the same run from
the repository root, from its own directory, or from a CI checkout elsewhere.

### Is the package installable on a clean machine?

The classic "works on my machine" bug: a dependency you have installed
and your users don't.

```yaml
suite: clean install
nodes:
  - name: clean
    type: lxd                            # boots an init system, so services can start
    options: { image: ubuntu:24.04 }
setup:
  - name: copy the built package in
    node: clean
    step:
      type: file_push
      # source is read on the machine running DART, relative to the suite file
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

Note: `service_status` runs `systemctl is-active <service>` on the node and
compares the output to `evaluate.status` (default `active`), so it requires
systemd on the target. LXD/Incus containers and VMs and SSH hosts qualify;
docker nodes generally do not, since standard base images carry no
`systemctl` and DART does not boot containers under an init system. The
equivalent check on a docker node is `type: execute` running the service's
own health or version command.

Run the same suite against several nodes at once by listing them:
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

Reachability is a property of *where you stand*. `from: node` — the default,
spelled out here for clarity — probes from the machine that matters, and
asserting `closed` proves a rule works.

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

Warning: teardown steps do not run when a run aborts. An unhandled setup
step failure, a test that errors, and `--stop-on-error` on a failing test
all skip straight to node and platform teardown, so a rollback placed in
`teardown:` does not execute. This is harmless for ephemeral nodes that are
deleted at teardown, but a long-lived node — an SSH host, or a container
kept across runs — stays in its broken state. Suites that depend on a
teardown rollback are best run without `--stop-on-error`. A test that
merely fails is not an abort: without `--stop-on-error` the run continues
and teardown steps do run.

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
  working-directory: tests            # local step paths resolve from here
  run: dart -c integration.yaml --report junit:results.xml
- name: Publish results
  uses: dorny/test-reporter@v1
  if: always()
  with:
    path: tests/results.xml
    reporter: java-junit
    fail-on-empty: false              # a run that aborts early writes no report
```

Note: report files are written on every exit from the test phase onward, so
a failed test, a `--stop-on-error` abort, an `--until` exit, or a teardown
failure all still produce the artifact. A run that aborts *before* the test
phase writes no report at all — a platform or node setup failure, a
fact-gathering error, a step or test construction error, and a failing setup
step all return early, as do `--setup-only` and an `--until` target inside
setup. Pipeline steps that consume the report must tolerate its absence,
which is what `fail-on-empty: false` above does.

Useful flags for pipelines:

| Flag | Why |
|---|---|
| `--check` | Validate the suite without touching infrastructure — a fast pre-commit lint, with the limits noted below |
| `--report junit:PATH` | Per-test results for GitHub/GitLab/Jenkins test panels |
| `--vars key=value` | Point one suite at staging or prod without editing YAML; comma-separated, and values may not themselves contain commas |
| `--only tag=smoke` | Run a subset; `--skip tag=slow` excludes. A filter that excludes every test is an error and exits non-zero |
| `-s`, `--stop-on-error` | Stop at the first failure |
| `-d`, `--debug` | Stream command output live while debugging a suite |

One limit on `--check` is worth knowing before it is wired into a pre-commit
hook: it validates only what step construction can check without a live node.
`file_template` reads and parses its source when the step is built, so a
missing or broken template is caught, whereas a missing `file_push` source is
not — that surfaces mid-setup, after nodes exist. The stand-in node it
substitutes for each declared node implements exactly that type's real
capabilities, so `reboot` and `snapshot` steps are accepted or rejected
exactly as a real run would.

Parameterize suites so one file serves every environment:

```yaml
suite: api smoke
vars:
  target: staging.internal      # dart -c suite.yaml --vars target=prod.internal
nodes:
  - name: local
    type: local
tests:
  - name: api answers
    node: local
    type: http_request
    tags: [smoke]
    options: { url: "https://{{var.target}}/health" }
```

---

## Splitting a suite across files

A large suite can keep its nodes, setup, tests, and teardown in separate
directories and pull them together with the `!!load_from` directive:

```yaml
suite: Organized Test Suite Example
docker:   !!load_from(docker)
nodes:    !!load_from(nodes)
setup:    !!load_from(setup)
teardown: !!load_from(teardown)
tests:    !!load_from(tests)
```

`!!load_from(<dir>)` is a textual preprocessor directive, not a YAML tag.
The directory is resolved relative to the suite file, not the working
directory. DART walks it recursively and concatenates the raw bytes of every
`.yaml` and `.yml` file it finds, in lexical order per directory, then
splices the result in beneath the directive line indented by two spaces.
Numeric filename prefixes such as `01_setup.yaml` are the convention for
pinning order.

The splicing rule shapes how fragments must be written. Only the text before
`!!load_from(` survives on that line — anything after the closing
parenthesis is discarded — and the loaded content becomes the block body one
level in. Fragments for `nodes`, `setup`, `teardown`, and `tests` are
therefore top-level sequences of `- name: ...` items, and a fragment for
`docker` is top-level mapping keys such as `networks:` and `images:`.
Individual fragments are not valid suites on their own and cannot be passed
to `dart -c`.

The pass runs once over the top-level suite file, before `{{var.*}}` and
`{{env.*}}` substitution and before the YAML is parsed. Loaded content is
subject to variable substitution but is never rescanned, so a `!!load_from`
inside a fragment is not expanded. A directive missing its closing
parenthesis fails with `malformed !!load_from directive on line N`, and a
missing or unreadable directory fails the load.

Warning: once a suite uses `!!load_from`, DART stops recording source
locations for it, so configuration errors print the message alone — no file
name, line number, or highlighted snippet. Rationale: inlining shifts line
numbers away from the files on disk, and a snippet pointing at the wrong
line is worse than none.

A worked example lives in
[`examples/merged/`](examples/merged/) — `main.yaml` is the entry point, the
`docker/`, `nodes/`, `setup/`, `teardown/`, and `tests/` directories hold its
fragments, and `dockerfiles/` holds the image build inputs they reference, and
the sibling directories hold its fragments.

---

## Documentation

The **[documentation site](https://bgrewell.github.io/dart/)** has
everything in a searchable form. The same pages live in the repository:

- **[Node types](docs/node-types.md)** — local, Docker, Docker Compose,
  LXD/Incus, SSH; remote daemons, ISO boot, security defaults
- **[Test types](docs/tests.md)** — every test type plus retries,
  timeouts, skips, captures, variables, and tags
- **[Evaluation reference](docs/evaluation.md)** — every `evaluate`
  check available
- **[Steps](docs/steps.md)** — setup and teardown: commands, packages,
  files, templates, snapshots, waits
- **[Command line](docs/cli.md)** — all flags and exit codes

Runnable examples live in [`examples/`](examples/) —
[`examples/basic/basic.yaml`](examples/basic/basic.yaml) needs no external
dependencies and is the place to start. Note: the files under
`examples/test_types/` and the YAML files in `examples/merged/`'s fragment
directories are `!!load_from` fragments rather than standalone suites, and
passing one to
`dart -c` fails to parse.

---

## How it works

A suite runs in phases:

1. **Platform setup** — create Docker networks/images, LXD projects/profiles
2. **Node setup** — start containers, connect to hosts
3. **Facts** — gather node facts (addresses are built in)
4. **Setup steps** — install, configure, wait for readiness
5. **Tests** — run and evaluate
6. **Teardown steps**, then **node** and **platform teardown**

Node and platform teardown run on every path that reaches the end of the run,
including after a failure: a deferred cleanup tears down every node that
completed setup, in declaration order, then every platform that completed
setup, in reverse, and prints `[+] cleaning up after error`.

Teardown *steps* (phase 6) are the exception — they run only when the test
phase completes. An unhandled setup step failure, a test that errors, and
`--stop-on-error` aborting on the first failing test all jump straight to
node and platform teardown.

Warning: `--setup-only` and `--until` with the default `exit` behavior stop
earlier by design and skip *every* teardown phase, leaving nodes and platforms
standing for inspection. `--teardown-only` is what removes them.

Report files follow the same boundary: everything from the test phase
onward produces a report, and anything that aborts before it produces none.
See [Running in CI](#running-in-ci) for what that means for a pipeline that
publishes results.

---

## Contributing

Issues and pull requests are welcome. `go test ./...` should pass and
`gofmt -l .` should be empty before you open one.

## License

Distributed under the license in [LICENSE](LICENSE).
