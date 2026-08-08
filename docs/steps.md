# Setup and Teardown Steps

Steps prepare and clean up the environment around your tests. Unlike tests they have no pass/fail conditions — a step either completes or fails the run.

[← Back to the README](../README.md)

Setup and teardown tasks in DART are specialized operations designed to prepare and clean up test environments. Unlike tests, which validate functionality and return pass/fail results, these tasks focus on environment management and are considered successful if they complete without errors.

### Purpose and Execution Flow

1. **Setup Tasks**
   - Run before any tests begin
   - Prepare the test environment (e.g., installing dependencies, configuring services)
   - Must complete successfully for tests to begin
   - Run in sequence to ensure proper initialization

2. **Teardown Tasks**
   - Run after all tests complete, including when tests fail — a failing test does not by itself skip teardown
   - Clean up resources and restore system state
   - Run in sequence; the first teardown step that fails aborts the remaining teardown steps
   - Skipped entirely when the run aborts early (see below)

Teardown steps do not run on every exit. A run that aborts returns before the
teardown-step phase. That happens when a platform or node fails to set up, when
a setup step fails, when a test's `skip_if` condition errors, when a test fails
under `--stop-on-error`, or when a test errors during execution (as opposed to
failing its evaluations). In those cases DART still runs its error cleanup —
node teardown followed by platform teardown, in reverse order, for the nodes and
platforms that had completed setup — but the configured `teardown:` steps are
skipped. `--setup-only` and `--until` (with the default `exit` behavior) end the
run without teardown steps *and* without node or platform teardown, leaving the
environment up.

Cleanup that must survive an aborted run therefore belongs in node and platform
teardown — container removal, network teardown — rather than in `teardown:`
steps. Note: `--setup-only` and `--until` with the default `exit` behavior skip
node and platform teardown as well, deliberately leaving the environment up for
inspection; `--teardown-only` is what removes it.

To run the teardown steps after an aborted run, invoke the suite again with
`--teardown-only`, which runs them best-effort: it continues past a failing step
and still tears down nodes and platforms. Note: that path gathers no facts, so
`{{ fact ... }}` references inside teardown steps are left unrendered — teardown
steps meant for this recovery route should not depend on facts.

### Unrecognised Options

An unrecognized key inside a step's `options:` is a configuration error naming
the offending key and the full accepted set, caught by `--check` before
anything is created:

```text
Error: unknown option "timout" in step "wait a moment"
(an execute step accepts: command, timeout)
```

Rationale: options were previously read by name and anything unread was
dropped in silence, so a misspelling left the option at its default while the
suite read as though it were set.

### When a teardown step fails

Teardown steps behave differently depending on how DART was invoked:

- **Normal run** — teardown steps run in order and the first failure aborts the
  rest. Later teardown steps are skipped and DART exits non-zero. Node and
  platform teardown still happen: the run falls into the error-cleanup path,
  which tears down every node and every platform that completed setup — nodes in
  declaration order, platforms in reverse — reporting failures without aborting.
- **`--teardown-only`** — teardown steps are best-effort. Every step runs even
  if earlier ones fail; failures are printed and the run continues into node and
  platform teardown. This mode exists for cleaning up after an aborted run,
  where some resources may already be gone.

Because a normal run stops at the first failure, order matters in a cleanup
chain: the steps that must always run belong first, or each step is written to
tolerate a missing resource. `file_delete` accepts `ignore_errors: true` so a
missing file is not a failure; for `execute` steps, appending `|| true` to a
command that may legitimately fail (for example `rm -rf /tmp/test-data || true`)
has the same effect. Note: `ignore_errors` is a `file_delete` option only, not a
generic step option.

### Key Differences from Tests

- **Success Criteria**: Tasks succeed/fail based on completion, while tests evaluate specific conditions
- **Evaluation**: Tasks don't have evaluation criteria like `match` or `contains`
- **Error Handling**: A failing step aborts the run, while test failures can be configured to continue. A setup-step abort also skips the configured teardown steps — only node and platform teardown run. See [When a teardown step fails](#when-a-teardown-step-fails) for how this differs between a normal run and `--teardown-only`.
- **Scope**: Tasks affect the environment, while tests validate functionality
- **Timing**: Tasks run before/after all tests, while tests run in the middle phase
- **Retries**: Steps have no `retry:` option; only tests do (see [Timeouts and Retries](tests.md#timeouts-and-retries))

### Targeting nodes

Every step requires a `node:`, and it accepts either a single node name or a
list of names:

```yaml
nodes:
  - name: web01
    type: docker
    options:
      image: ubuntu:22.04
  - name: web02
    type: docker
    options:
      image: ubuntu:22.04
  - name: web03
    type: docker
    options:
      image: ubuntu:22.04

setup:
  - name: install curl
    node: [web01, web02, web03]
    step:
      type: execute
      options:
        command: apt-get update && apt-get install -y curl
```

A list is expanded at config-load time into one identical step per node,
executed sequentially in the order the nodes are listed — the example above runs
as three separate `install curl` steps against web01, then web02, then web03.
The expanded copies share the step's `name`, so console output repeats the title
once per node, distinguished by the node column. The same syntax works in
`teardown:` and in `tests:` (see [Tests](tests.md); the `consistency` test type
is the one exception, keeping its node list intact instead of expanding it).
`examples/multi-node/` contains a runnable demonstration.

`node:` is mandatory even for steps that do not execute on the node.
`http_request`, `dns_request`, and `simulated` run on the machine hosting DART,
but still require a `node:` naming a node the suite declares. A step with a
missing or empty `node:` fails config loading with
`setup step "<name>" references no node` (or `teardown step ...`), and a name
that matches no entry under `nodes:` fails with
`node "<name>" not found (referenced in step "<name>")`.

### Available Task Types

#### Execute Task (`execute`)
Run shell commands on the target node. Ideal for custom setup operations.

```yaml
- name: configure database
  node: db-server
  step:
    type: execute
    options:
      command: |
        mysql -u root -e "CREATE DATABASE testdb;"
        mysql -u root -e "GRANT ALL ON testdb.* TO 'testuser'@'%';"
```

`command` accepts either a single string — including a YAML block scalar, as
above — or a list of strings. List entries run in order on the same node and the
step stops at the first entry that exits non-zero; the remaining entries do not
run. A non-string list entry, or a `command` that is neither a string nor a
list, is a configuration error rejected when steps are constructed — after
platform and node setup, before the first step runs (and at `--check` time).

```yaml
- name: prepare the database
  node: db-server
  step:
    type: execute
    options:
      timeout: 30          # seconds, per command; default 0 = wait forever
      command:
        - mysql -u root -e "CREATE DATABASE testdb;"
        - mysql -u root -e "GRANT ALL ON testdb.* TO 'testuser'@'%';"
```

A non-zero exit code fails the step — and therefore the run — reporting
`command failed with exit code N: <stderr>`. There is no `ignore_error` or
`allow_failure` option; a command that may legitimately fail needs `|| true`
appended.

`timeout:` is in seconds, accepts fractional values, and must be non-negative.
It bounds the wait for each command in the step individually, not the step as a
whole. Omitted or `0`, the wait is unbounded, so a hung command hangs the run.
[Timeouts and Retries](tests.md#timeouts-and-retries) covers the rest, including
the note that a timeout bounds only the suite-side wait while the remote process
may keep running.

#### APT Package Management (`apt`)
Install Debian and Ubuntu packages on the target node, refreshing the package
index first when it is stale.

```yaml
- name: install system dependencies
  node: test-container
  step:
    type: apt
    options:
      packages:
        - nginx
        - postgresql
        - redis-server
```

**Options**

| Option | Type | Required | Description |
|---|---|---|---|
| `packages` | list of strings | yes | Packages to install, passed to a single `apt-get install` in the order listed. Must be a non-empty YAML array of strings. |

`packages` is the step's only option.

The step issues up to three commands on the target node: a
`stat -c %Y /var/lib/apt/periodic/update-success-stamp` probe, then
`sudo -n apt-get update` when that probe says the index is stale, then
`sudo -n apt-get install -y <packages...>` always.

Warning: `sudo` is hardcoded with `-n` (never prompt), so the node must run as
root or grant the connecting user passwordless sudo for `apt-get`, and the
`sudo` binary must be present. Many base container images — including
`ubuntu:22.04`, which `examples/multi-node/docker-multi-node.yaml` uses — do not
ship the `sudo` package, in which case the step fails with `sudo: not found`
(exit 127), reported as `apt-get update failed: ...`. On an SSH node whose sudo
requires a password the failure reads `apt-get install failed: sudo: a password
is required`. The remedies are to bake `sudo` into the image or to use an
`execute` step running `apt-get` directly as root. Note: node-level sudo
configuration does not help here — Docker, LXD, and SSH nodes ignore
`exec_opts.sudo` entirely, and even a local node's configured sudo password
cannot satisfy `-n`, which makes sudo fail rather than read a password.

The index refresh follows a fixed rule with no override. Before installing, the
step reads the mtime of `/var/lib/apt/periodic/update-success-stamp` with
`stat -c %Y`. `apt-get update` runs when that stamp is older than 24 hours, and
also whenever it cannot be read at all — missing file, non-zero exit, or
unparsable value all count as stale. A fresh container typically has no stamp,
so the first `apt` step against it always updates. There is no option to force
or suppress the refresh.

The step installs only. Removal, purging, upgrades, holds, version pinning,
repository configuration, and `DEBIAN_FRONTEND` are all out of scope, and the
step exposes no `timeout` — an `execute` step covers anything beyond
installation. Package names are passed to the shell unquoted, so a `name=version`
pin works only if it is shell-safe.

#### Simulated Task (`simulated`)
Add controlled delays in the setup/teardown process. Useful for:
- Waiting for services to initialize
- Simulating network delays
- Testing timing-dependent scenarios

```yaml
- name: wait for service initialization
  node: app-server
  step:
    type: simulated
    options:
      time: 5                        # Seconds; fractional values like 0.5 are allowed
      message: waiting for the API   # Optional; shown while the step waits
```

`message` is displayed as the step's status for the duration of the delay,
describing what is being stood in for. The delay elapses on the machine
running DART; the named node is used only to
label the step in console output and reports. `node:` is still required and must
name a declared node — see [Targeting nodes](#targeting-nodes). A fixed sleep is
a blunt instrument, so waiting on an observable condition with `wait_for` is
generally preferable.

#### Wait For (`wait_for`)
Poll a command on the target node until it exits zero. This is the convergence
primitive for setup: it holds the run until a service is actually answering,
rather than guessing at a delay.

```yaml
- name: wait for the API to answer
  node: app-server
  step:
    type: wait_for
    options:
      command: curl -sf http://localhost:8080/health
      timeout: 120   # seconds, default 60; must be positive
      interval: 3    # seconds between polls, default 2
```

Options:

- `command` — required, non-empty. Runs on the step's node; exit code zero means
  ready.
- `timeout` — seconds to keep polling. Default `60`; fractional values are
  allowed and the value must be greater than zero.
- `interval` — seconds between polls. Default `2`; fractional values are
  allowed and the value must be greater than zero.

Polling never overlaps invocations: the command is awaited in interval-sized
slices against a single in-flight invocation, so a check that runs longer than
the interval is re-awaited rather than launched again. Consequently the step can
overrun its deadline by at most one interval. On timeout the step fails with
`wait_for timed out after <timeout>: <command>`.

#### File Operations (`file_create`, `file_edit`, `file_delete`, `file_exists`, `file_read`)
Create, modify, verify, and remove files **on the step's target node** —
local nodes use the native filesystem; container and SSH nodes are driven
through their shell, which must provide `sh`, `cat`, `test`, `rm`, `mkdir`,
`chmod`, `printf`, `base64`, and `stat`. `file_write` is an alias for
`file_create`.

Note: `base64` and `stat` are not POSIX utilities. File contents travel
base64-encoded (`printf '%s' <chunk> | base64 -d`), so an image without `base64`
— distroless, scratch, or a stripped busybox without the applet — fails every
write step (`file_create`, `file_edit`, `file_push`, `file_template`) with
`command failed with exit code 127`; the node's own `base64: not found` is
carried through in the error when the node captures stderr. Read, existence, and
delete steps need only `cat`, `test`, and `rm`, and still work. Permission reads
use GNU syntax (`stat -c %a`); on a node with a BSD or macOS userland, where
`stat` needs `-f %Lp`, `file_edit` does not error — it falls back to `0644`, so
an edited file silently loses its original permissions. Setting `mode:`
explicitly avoids that on such nodes.

```yaml
- name: write app config
  node: test-container
  step:
    type: file_create
    options:
      path: /etc/myapp/config.ini
      contents: |
        [server]
        port = 8080
      create_dir: true       # mkdir -p the parent directory
      overwrite: true        # without this, an existing file is an error
      mode: "0640"           # always a quoted octal string — see note below

- name: require the config to be present
  node: test-container
  step:
    type: file_exists
    options:
      path: /etc/myapp/config.ini   # the only option, and it is required

- name: point app at test database
  node: test-container
  step:
    type: file_edit
    options:
      path: /etc/myapp/config.ini
      operation: replace     # insert | replace | remove
      match_type: plain      # plain | regex (replace/remove); insert also supports line
      match: "port = 8080"
      content: "port = 9090"
      # insert also takes position: before|after, and match_type: line with line_number;
      # regex replace expands $1 / ${name} by default; write $$ for a literal $
      # replace and remove rewrite EVERY match in the file; insert acts on the
      # first match only. There is no count/first_only option — the match
      # pattern has to be specific enough if only one occurrence should change.

- name: verify config content
  node: test-container
  step:
    type: file_read
    options:
      path: /etc/myapp/config.ini
      contains: "port = 9090"   # optional; plain substring, not a regex

- name: cleanup
  node: test-container
  step:
    type: file_delete
    options:
      path: /etc/myapp/config.ini
      ignore_errors: true    # complete the step regardless of why the delete failed
```

**Always quote `mode` as an octal string.** `"0640"`, `"644"`, and `"0o644"` all
parse as octal. An unquoted `mode: 0644` also works, because YAML reads
leading-zero integer literals as octal — but an unquoted value *without* the
leading zero is a trap, since nothing distinguishes a decimal literal from an
octal one. Bare integers above `0o777` (511 decimal) are rejected with a config
error, so `mode: 644` fails loudly; bare integers at or below 511 are taken at
face value and fail silently — `mode: 444` becomes decimal 444, that is `0o674`
(`rw-rwxr--`), not the intended `r--r--r--`. Setuid, setgid, and sticky modes
require the string form: `mode: "1777"`, `mode: "4755"`. The leading-zero literal
`mode: 01777` is *also* rejected, because it resolves to 1023, above the `0o777`
integer limit.

`mode` is optional on `file_create`/`file_write` and `file_template`. Omitted, a
newly created file lands at 0644 masked by the umask in effect (on container and
SSH nodes the file is created by the node's shell, so that node's umask applies),
and an existing file being overwritten keeps whatever permissions it already had
— the write truncates the contents but does not reset the mode. When `mode` is
given it is applied with a `chmod` after the write, so the resulting permissions
are exactly what was requested on both new and existing files and the umask does
not apply. `file_push` differs: with no `mode` it carries the source file's
permission bits, so pushed scripts stay executable. `file_edit` has no `mode`
option and preserves the file's existing permissions, falling back to 0644 only
when they cannot be read. `file_fetch` also has no `mode` option; the local
destination is created 0644 subject to the umask, and an existing destination
keeps its mode.

**`file_edit` requires its match to be present.** With `match_type: plain` or
`regex`, a pattern that is not in the file fails the step with
`match not found: <pattern>` or `regex match not found: <pattern>`. A missing
match is never a silent no-op, and `file_edit` has no `ignore_errors` option
(unlike `file_delete`). A failing setup step ends the run unless
`--pause-on-error` is used and "continue" is chosen; a failing teardown step
always ends the run.

**`match_type: line` is implemented for `operation: insert` only**, where it
inserts relative to `line_number` and an out-of-range line fails with
`line number N is out of range (1-M)`. `replace` and `remove` support `plain` and
`regex`. Note: a `replace` or `remove` step with `match_type: line` passes
configuration validation and `--check` — validation only requires
`line_number >= 1`, and `--check` constructs steps without running them — and
then fails when the step runs with `unsupported match type for replace: line` or
`unsupported match type for remove: line`.

**Match cardinality differs by operation and is not configurable.** `replace` and
`remove` act on every match in the file. `insert` with `match_type: plain` or
`regex` acts on the first match only. There is no `count` or `first_only`
option; narrowing the `match` pattern is the way to limit a substitution.

**Regex replace and the `$` character.** With `match_type: regex` and
`operation: replace`, `content` is a Go `regexp` replacement template, not a
literal string. `$1`, `${1}`, and `${name}` expand to capture groups and `$0`
expands to the whole match — this happens by default, without `use_captures`.

Warning: any `$` in `content` followed by a letter, digit, or `_` (or by `{`) is
treated as a group reference, and a reference to a group that does not exist
expands to the empty string. `content: "PATH=$PATH:/opt"` writes `PATH=:/opt`,
and `content: "price=$5.00"` writes `price=.00`, with the step still reporting
success. A literal dollar sign is escaped as `$$`. A `$` followed by anything
else — a space, a `.`, the end of the string — is already left alone.

`use_captures: true` selects a different, non-template substitution: each match
is processed individually, `${name}` placeholders are replaced by plain string
substitution first, then `${N}` and `$N` from the highest group index down to
`$0`. The practical differences are that `$` is left alone unless it forms a
reference to a group the pattern actually defines, `$$` is *not* an escape (it
stays a literal `$$` unless it happens to form a group reference), `$name`
without braces is not recognized for named groups (only `${name}` is), and text
pulled in from an earlier group can itself be rescanned by later substitutions.
The default path with `$$` escaping is preferable unless the replacement text
must contain unescaped `$`.

Note: this applies only to `operation: replace` with `match_type: regex`.
`operation: insert` inserts `content` verbatim, plain-match `replace` substitutes
verbatim, and `operation: remove` replaces with the empty string, so none of them
expand `$`.

`file_exists` takes a single required option, `path`, and no others. The step
succeeds when the path is present on the target node and fails with
`file does not exist: <path>` when it is absent; a stat or shell failure while
checking reports `error checking file: ...`. Because a failing setup step aborts
the run (unless `--pause-on-error` is used to skip or retry it), `file_exists`
serves as a cheap precondition assertion. Note: existence is tested with
`os.Stat` on local nodes and `test -e` on container and SSH nodes, so a directory
at `path` also satisfies the step.

`file_read`'s `contains` is optional and is matched as a literal substring, not a
regular expression. Omitted, `file_read` only asserts that the file can be read —
it fails with `failed to read file: ...` when the read fails and otherwise passes
regardless of content. A mismatch fails with
`file content validation failed: expected content missing`.

`ignore_errors` on `file_delete` is not limited to a missing file: the step marks
itself complete whenever the delete returns any error — missing path, permission
denied, read-only filesystem, or the path being a directory. A real failure never
surfaces once this is set. Deletion is also non-recursive: container and SSH
nodes run `rm -- <path>` with no `-r` or `-f`, so a directory always fails and a
missing file is an error unless `ignore_errors` is set. Local nodes use
`os.Remove`, which removes an empty directory but fails on a non-empty one.

#### File Transfer and Templating (`file_push`, `file_fetch`, `file_template`)
Deploy repository files onto a node, pull artifacts back, or render one
template per node. Local nodes use the filesystem directly; container and
SSH nodes are driven through their shell, with the same utility requirements
described under File Operations above.

```yaml
setup:
  - name: deploy the service binary
    node: app-server
    step:
      type: file_push
      options:
        source: build/myservice          # path on the machine running DART
        dest: /usr/local/bin/myservice
        overwrite: true
        create_dir: true
        # mode defaults to the source file's permission bits

  - name: render per-node config
    node: app-server
    step:
      type: file_template
      options:
        source: fixtures/app.conf.tmpl   # Go template: {{ .port }}
        dest: /etc/myapp/app.conf
        overwrite: true
        mode: "0640"
        create_dir: true                 # mkdir -p the parent on the node
        values:
          port: 8080
          backend: "{{ fact \"db\" \"ipv4\" }}"

teardown:
  - name: keep the logs for triage
    node: app-server
    step:
      type: file_fetch
      options:
        source: /var/log/myapp.log
        dest: artifacts/myapp.log
        create_dir: true
```

Templates are parsed when the step is constructed, so a broken one fails before
any step runs, and a value that is missing or null is an error rather than a
silently empty (or literal `<no value>`) config line. `file_fetch` refuses
to overwrite an existing local file unless `overwrite: true`, so a fetched
artifact cannot clobber a previous run's.

`file_push`, `file_template`, and `file_create` take the same three write
options — `mode`, `overwrite`, and `create_dir`, where `create_dir` runs
`mkdir -p` on the destination's parent *on the node*. `file_fetch` takes only
`overwrite` and `create_dir`, and there `create_dir` creates the parent directory
*locally*. `file_fetch` has no `mode` option: the local file is created 0644
subject to the umask, so a fetched executable arrives without its executable
bit, and an existing destination overwritten with `overwrite: true` keeps its
current mode. A following `execute` step with `chmod` covers the cases where that
is not what is wanted.

Local paths in these steps — `source` for `file_push` and `file_template`,
`dest` for `file_fetch` — follow the same rule as every other local path a
suite writes: absolute paths are used as-is, `~` expands to the invoking user's
home directory, and anything else is relative to the directory holding the
suite file. Running `dart -c examples/foo/suite.yaml` from the repository root
therefore reads `fixtures/app.conf.tmpl` at `examples/foo/fixtures/app.conf.tmpl`,
and the same command works unchanged from any directory.

Note: a missing source still surfaces at different times by step type. A
missing `file_template` source fails at step construction with
`cannot read template <path> in step "<name>"`, before any step runs, while a
missing `file_push` source fails mid-run with `failed to read source <path>`.

Content to container and SSH nodes is written in 32 KiB base64 chunks, so files
are not limited by the shell's per-argument size cap. That write is not atomic:
the first chunk truncates the destination and later chunks append, so a failure
part-way through leaves a partial file in place — the error names the failing
chunk's offset into the encoded payload. The requested `mode` is applied only
after the whole write succeeds, so a partial file keeps whatever mode it already
had. When atomicity matters — replacing a running service binary or a live
config — writing to a temporary path and moving it into place with an `execute`
step is the safe pattern. Note: local nodes are not chunked; they use a single
truncating write, which is also not atomic.

#### Snapshots (`snapshot`)
Give destructive tests cheap isolation on LXD nodes: capture state in
setup, break things, roll back in teardown — far faster than recreating
a node.

```yaml
setup:
  - name: capture clean state
    node: iso-vm
    step:
      type: snapshot
      options:
        name: clean          # action defaults to create
        # stateful: true     # include running memory (needs CRIU)
```

Restoring an instance that was running stops and restarts it, and DART blocks
until the node accepts commands again, so a following step cannot race the
reboot. An instance that was already stopped is restored in place and stays
stopped — there is no readiness wait.

```yaml
teardown:
  - name: roll back
    node: iso-vm
    step:
      type: snapshot
      options: { name: clean, action: restore }
      # A snapshot taken with stateful: true must also be restored with
      # stateful: true — otherwise LXD performs a disk-only restore and
      # silently discards the saved memory.
  - name: clean up the snapshot
    node: iso-vm
    step:
      type: snapshot
      options: { name: clean, action: delete }
```

`stateful` applies to `create` and `restore` only; combining it with
`action: delete` is rejected when the step is constructed, before any step runs.
`action: delete` is idempotent — the delete is issued and a not-found response is
tolerated, so teardown is safe to re-run or to run after a restore that already
consumed the snapshot. Note that DART does not verify the snapshot existed before
deleting it.

Note: `--check` currently rejects any suite containing a `snapshot` step, failing
with `node "<name>" does not support snapshots (supported: lxd) in step "<name>"` even when the
node really is an LXD instance. The `--check` harness substitutes a mock for
every node and that mock does not implement the snapshot capability; it does
implement reboot, so `reboot` steps and tests validate normally. On a real run
the capability check happens during step construction, after platform and node
setup and fact gathering — so a suite that uses snapshots has to be validated by
running it rather than by `--check`.

#### Reboot (`reboot`)
Restart the target node and block until it accepts commands again, so a
following step cannot race the reboot. Frequently paired with `snapshot` in
crash-safety suites.

```yaml
setup:
  - name: reboot to apply the new kernel
    node: iso-vm
    step:
      type: reboot
      options:
        mode: graceful          # 'force' models a power cut
        ready_command: cat /etc/hostname
        timeout: 600            # seconds; 0/omitted uses the node's default
```

Options:

- `mode` — `graceful` (the default) or `force`. Any other value is a config
  error. `force` kills the instance without a clean shutdown (LXD `Force: true`)
  or adds `-f` to the remote reboot command on SSH.
- `ready_command` — optional override of the node's readiness check. On LXD it
  replaces the `boot_wait` ready command; on SSH it defaults to `true`, so a bare
  successful connection counts as ready.
- `timeout` — seconds to wait for readiness. Negative values are a config error.
  Omitted or `0` means: on LXD, reuse the node's `boot_wait` timeout; on SSH,
  wait up to five minutes.

The node must support rebooting, which only `lxd` and `ssh` nodes do. A node type
that cannot reboot fails when the step is constructed — after platform and node
setup, before the first setup step runs — with
`node "<name>" does not support reboot (supported: lxd, ssh) in step "<name>"`.
Warning: `--check` does not catch this. Its mock node implements reboot, so a
suite that reboots a docker node validates clean and only fails on a real run,
after the containers have been created. `reboot` is also available as a test type; see [Tests](tests.md), which
covers the LXD and SSH readiness behaviour in more detail and notes that `retry:`
is rejected on `reboot` tests.

#### Service Check (`service_check`)
Verify a systemd service is active on the target node.

```yaml
- name: ensure nginx is running
  node: web-server
  step:
    type: service_check
    options:
      service: nginx
```

#### HTTP Request (`http_request`)
Perform an HTTP request and validate the response. The request is made from
the step's node, so it verifies what that node can reach.

```yaml
nodes:
  - name: app
    type: docker
    options:
      image: myapp:latest

setup:
  - name: check API health endpoint
    node: app
    step:
      type: http_request
      options:
        url: http://localhost:8080/health
        method: GET              # default GET
        expected_status: 200     # default 200
        expected_body: healthy   # optional substring check
        timeout: 5               # seconds, default 30; must be > 0
        from: node               # default node; host asks from the controller
        headers:                 # optional request headers
          Authorization: Bearer {{ var.token }}
```

`from` chooses the vantage point:

- `node` (the default) issues the request on the node with `curl`, which the
  node's image must provide. A missing `curl` fails the step with
  `curl is required on this node`, rather than reading as an unreachable
  endpoint.
- `host` issues it from the machine running DART with Go's HTTP client. Use it
  for endpoints that must be reachable from CI, or for a published container
  port, or when the node's image ships no `curl`.

Note: `localhost` means different things from each vantage. From the node it is
the node's own loopback; from the host it is the controller's, reaching a
container only through a published port.

`node:` is required and validated like any other step's — see
[Targeting nodes](#targeting-nodes). With `from: host` the node is used only to
label the step in console output and reports, but it is still provisioned and
torn down by the suite, so referencing a node the suite already declares is
preferable to adding one solely for these steps.

Unlike `execute` and `reboot`, where `timeout: 0` or an omitted timeout means
"no bound", `http_request` rejects `timeout: 0` when the step is constructed with
`timeout must be positive in step "<name>"` — the value must be greater than
zero. `dns_request` rejects it the same way.

Validation is deliberately narrow: `expected_status` is a single exact status
code, with no ranges and no lists, and `expected_body` is a plain substring match
against the entire response body (skipped when empty) rather than a regex or a
JSON path. Redirects are followed either way — `curl --location` on the node,
Go's default client (ten hops) from the host — and the status compared is the
final response's.

The step sends no request body and no authentication, and TLS certificate
verification cannot be disabled. When either is needed, an `execute` step
invoking `curl` on the node covers it; for richer assertions the `http_request`
*test* type accepts the full `evaluate` set (see [Tests](tests.md)).

#### DNS Request (`dns_request`)
Resolve a hostname and optionally verify expected addresses appear in the
answers. Resolution happens on the step's node, using that node's resolver and
hosts file — which is usually the question worth asking, since a container's
view of a name routinely differs from the controller's.

```yaml
nodes:
  - name: app
    type: docker
    options:
      image: myapp:latest

setup:
  - name: verify service DNS
    node: app
    step:
      type: dns_request
      options:
        hostname: db.test.internal
        expected_ips:
          - 10.0.0.5
        timeout: 10      # seconds, default 10; must be > 0
        from: node       # default node; host resolves on the controller
```

On the node the lookup uses whichever of `getent`, `dig`, `host`, or `nslookup`
the image provides, in that order — `getent` first because it consults
`nsswitch.conf`, so `/etc/hosts` entries and resolver plugins count exactly as
they would for the node's own applications. An image with none of them fails the
step with a message naming all four, rather than reporting the name as
unresolvable. A name that resolves to nothing is likewise an error, not an empty
pass.

With `from: host` the lookup uses Go's resolver on the machine running DART, and
the `node:` reference only labels the step — but it is still required and must
name a declared node. See [Targeting nodes](#targeting-nodes).

### Best Practices

1. **Environment Isolation**
   - Use setup tasks to create isolated test environments
   - Ensure teardown tasks clean up ALL created resources
   - Avoid leaving behind test artifacts

2. **Idempotency**
   - Design tasks to be repeatable
   - Handle cases where resources may already exist
   - Ensure clean state regardless of previous runs
   - Note: `file_edit` is not idempotent by default. Once `port = 8080` has been
     rewritten to `port = 9090`, re-running the same replace fails, because the
     original text is gone. Repeated edits are made safe by writing the file
     fresh first with `file_create` and `overwrite: true` so each run starts from
     a known state (which is what the File Operations example above does), by
     matching on a stable anchor that survives the edit — `match_type: regex`
     with `^port = .*` and `content: "port = 9090"` — or by using a regex that
     matches both the pre- and post-edit forms, such as `port = (8080|9090)`.

3. **Error Handling**
   - Include error checking in setup tasks
   - Implement proper cleanup in teardown tasks
   - Log relevant information for debugging

4. **Resource Management**
   ```yaml
   setup:
     - name: create test directory
       node: test-server
       step:
         type: execute
         options:
           command: "mkdir -p /tmp/test-data"
   
   teardown:
     - name: cleanup test directory
       node: test-server
       step:
         type: execute
         options:
           command: "rm -rf /tmp/test-data || true"
   ```

### Planned Future Task Types

DART is actively developing additional task types to enhance environment management:

- **SNAP Package Management**
  - Install/remove snap packages
  - Configure snap services

- **Git Operations**
  - Clone repositories
  - Checkout specific branches/tags
  - Apply patches

- **Network Configuration**
  - Configure network interfaces
  - Set up routing rules
  - Manage firewall settings

- **Service Management**
  - Start/stop system services
  - Configure service parameters
  - Manage service dependencies

---
