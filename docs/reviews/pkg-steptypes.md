# Review: setup/teardown steps (`pkg/steptypes`)

- **Scope:** `pkg/steptypes/` (all step implementations and the `CreateSteps`
  factory), the `MockNode` in `pkg/nodetypes/mock.go`, step-related docs.
- **Started:** 2026-08-06
- **Status:** complete (2026-08-06)

## Bugs

- [x] **Six step types implemented but unreachable.**
  `file_exists`, `file_read`, `file_write`, `http_request`, `dns_request`,
  and `service_check` exist with implementations and unit tests — and are
  listed in the docs — but were never added to the `CreateSteps` switch, so
  any config referencing them fails with "unknown step type".
  *Fix:* all step types wired through a factory registry with validated
  option parsing.

- [x] **File steps operate on the DART host, not the target node.**
  `file_create`, `file_delete`, `file_edit` (and the unwired file steps) use
  `os.*` directly and ignore the node passed to their factories. With a
  docker/LXD/SSH node the step silently modifies the controller's filesystem
  while the config says `node: <container>`.
  *Fix:* a `fileOps` abstraction — direct filesystem access for local nodes,
  shell-based operations (via `node.Execute`, base64-encoded content) for
  remote nodes.

- [x] **`ExecuteStep` failure messages print a reader struct.**
  `fmt.Errorf("...: %s", result.Stderr)` formats the `io.Reader` itself, not
  its content.
  *Fix:* read stderr through the cached `StderrBytes()` accessor.

- [x] **`service_check` unit tests use the wrong mock API** (pre-existing
  failures). They construct `&nodetypes.MockNode{}` (nil maps) and set
  testify `.On(...)` expectations that the map-based mock never consults.
  *Fix:* tests rewritten against `NewMockNode()`/`SetResponse`.

- [x] **`MockNode.SetResponse` stores one-shot readers.**
  A command executed more than once returns drained output on subsequent
  calls. *Fix:* the mock stores strings and mints a fresh
  `ExecutionResult` per `Execute` call.

- [x] **`AptStep.AptUpdateNeeded` date parsing is dead code.**
  It splits the `stat` output on spaces and parses only the date portion
  against a datetime layout, which always fails — so `apt-get update` runs
  on every apt step. *Fix:* use `stat -c %Y` (epoch seconds).

- [x] **`simulated` panics on bad config.**
  `Options["time"].(int)` panics when the key is missing or not an int
  (e.g. `time: 2.5`). *Fix:* validated parsing accepting integer or
  fractional seconds.

- [x] **Bare-decimal `mode:` values yield the wrong permissions.**
  Verified during the fix: yaml.v3 parses leading-zero literals as octal
  (`0644` → `0o644`), so those were already correct — but a bare
  `mode: 644` arrives as decimal 644 (`0o1204`) and previously became
  broken permissions silently. *Fix:* strings parse as octal (`"644"`,
  `"0644"`, `"0o644"`, recommended form); integers are accepted directly up
  to `0o777`, and anything larger — the signature of a bare decimal or a
  special-bit mode — is a config error with guidance. Residual caveat: a
  bare decimal ≤ `0o777` (e.g. `mode: 444`) is indistinguishable from a
  correct value and is taken at face value.

- [x] **`FileEditStep` clobbers file permissions.**
  The edited file was always written back with `0644`. *Fix:* the original
  mode is captured before the edit and restored on write (best-effort).

- [x] **Silent option-type coercion in factories.**
  Blank-identifier assertions (`contents, _ := ...(string)`) meant a
  wrong-typed option silently became its zero value (`contents: 123` → empty
  file, `overwrite: "yes"` → false). *Fix:* shared option helpers return
  config errors with location information.

## Improvements

- [x] **Step factory registry.**
  The `CreateSteps` switch is replaced by a `type → factory` registry
  (mirroring `internal/eval`), so new step types are one file plus one
  registry entry. Step type names become constants.

- [x] **Remove the `BaseStep.Run` placeholder.**
  It returned `nil`, so a step type that forgot to implement `Run` would
  silently succeed. Removed; the compiler now enforces `Run` per step.

- [x] **`file_write` folded into `file_create`.**
  `FileWriteStep` was a strict subset of `FileCreateStep` (no mode, no
  create_dir). `file_write` is now an alias for the same factory; the
  duplicate implementation is removed.

- [x] **Steps read output via cached accessors.**
  `service_check` and `apt` now use `StdoutBytes()`/`StderrBytes()` instead
  of raw `io.ReadAll`, consistent with the evaluator fix in
  [internal-eval.md](internal-eval.md).

- [x] **Command arguments shell-quoted.**
  Paths and service names are quoted before interpolation into shell
  commands (`systemctl is-active`, `cat`, `rm`, …), so names with spaces or
  metacharacters behave.

## Gaps

- [x] `http_request` has no timeout default/option parsing wired; factory
  now parses `method` (default GET), `expected_status` (default 200),
  `expected_body`, and `timeout` seconds (default 30).
  Note: the request runs from the DART host, not the node — documented.
- [x] `dns_request` factory parses `hostname` (required) and `expected_ips`
  (optional list). Resolution happens from the DART host — documented.
- [x] Future (not in this pass): request headers/body for `http_request`,
  custom nameserver for `dns_request`, `should_exist: false` form for
  `file_exists`, Windows-node support for shell-based remote file ops
  (remote file ops assume a POSIX shell; local nodes use the native
  filesystem path and are unaffected).

## Resolution log

- **2026-08-06** — All findings resolved in one pass on branch
  `improve/steptypes-review`:
  - `CreateSteps` rebuilt around a `type → factory` registry; all 12 step
    types (including the six previously unreachable ones) construct through
    validated factories with shared option helpers (`optString`, `optBool`,
    `optInt`, `optFloat`, `optStringList`, `optFileMode`).
  - New `fileOps` abstraction (`fileops.go`): `localFileOps` (native
    filesystem, used for local/nil nodes) and `execFileOps` (shell commands
    over `node.Execute`, base64-encoded content, quoted paths). Verified
    that docker (`sh -c`), LXD (`bash -c`), and SSH nodes all interpret
    commands through a shell.
  - `FileWriteStep` deleted; `file_write` registered as an alias of
    `file_create`.
  - `MockNode` rewritten to store canned output as strings and mint fresh
    readers per `Execute`; the misleading unused testify embed removed. The
    two failing `service_check` tests rewritten against the real mock API.
  - Unit tests added: `fileops_test.go` (both implementations run the same
    behavior suite; the exec variant against a real `/bin/sh`-backed local
    node), `factory_test.go` (wiring, validation, mode parsing),
    `apt_test.go` (staleness logic, execute stderr formatting).
  - End-to-end smoke run exercised `file_create` (with mode), `file_edit`
    (mode preserved), `file_exists`, `file_read`, `dns_request`,
    `service_check`, fractional `simulated`, and `file_delete`. The smoke
    test caught the yaml.v3 octal subtlety, which led to the final mode
    rules above.
  - README: file/service/HTTP/DNS step types documented under "Available
    Task Types"; the stale "File System Operations" planned-work bullet
    removed.
