---
name: project-dart-overview
description: DART repo layout, test harness conventions, and where the retry/timeout/reboot machinery lives.
metadata:
  type: project
---

DART (`github.com/bgrewell/dart`, repo at `/home/ben/repos/bgrewell/dart`) is a
distributed test-scenario runner. Core pieces relevant to review work:

- `pkg/ifaces/node.go` — `Node` interface (`Setup/Teardown/Execute/Close`) plus
  the free function `ExecuteWithTimeout(node, command, timeout)`, which
  goroutine+select bounds a `node.Execute` call. Zero/negative timeout = call
  `node.Execute` directly, no goroutine spawned.
- `pkg/testtypes/base.go` — `BaseTest.runProducer` is the shared test
  lifecycle: setup cmds (once) -> attempt loop (retry) -> teardown cmds
  (once, always) -> evaluate. `BaseTest.attempt()` does one produce+capture+
  evaluate cycle. `CreateTests` wires `cfg.Retry` onto `BaseTest` generically
  for every test type (execute, reboot, http_request, etc.) — there is no
  per-test-type opt-out.
- `pkg/testtypes/reboot.go` — `RebootTest.Run` calls `rebooter.Reboot(...)`
  through the same generic `runProducer`, so it is not exempt from retry.
- `pkg/steptypes/wait_for.go` — polls a command via `ExecuteWithTimeout`,
  using **remaining time until its own deadline** as each attempt's timeout
  (not a fixed per-attempt slice, and not `min(interval, remaining)`).
- `pkg/nodetypes/mock.go` — `MockNode` used across `pkg/testtypes` and
  `pkg/steptypes` tests. `QueueResponse` gives one-shot FIFO responses per
  command string, consumed before the persistent `SetResponse` mapping.
  Guarded by one mutex; safe for concurrent callers but the FIFO order
  itself is a logical resource that concurrent callers can race over.
- Test helpers: `pkg/testtypes/testtypes_test.go` has `makeTest`/`runTest`/
  `allPassed`; `pkg/steptypes/factory_test.go` has `makeStep`. Retry-specific
  tests build `config.TestConfig` directly (see `pkg/testtypes/retry_test.go`
  `makeRetryTest`) since `makeTest` doesn't thread a `Retry` field through.
- `internal/controller.go` `Run()` (~line 482): a non-nil `runErr` from
  `test.Run()` causes an **unconditional** `return runErr` that aborts the
  whole suite run and skips the YAML-level `teardown:` step list (though
  node/platform teardown still happens via the top-level `defer`). This is
  pre-existing controller behavior, not new, but the retry/timeout feature
  makes it easy to hit routinely (see [[feedback-dart-retry-timeout-review]]).

See [[feedback-dart-retry-timeout-review]] for the specific bugs found in the
`feature/timeouts-retry` branch and how they were verified.

## CI integration (`feature/ci-integration`, on top of timeouts-retry)

- `internal/report/report.go` — `TestRecord`/`Report`/`Spec`/`ParseSpec`/
  `Write`/`FromRecords`. `Status` is one of pass/fail/skip/ran/error.
  `FromRecords` derives `Report` totals from a `[]TestRecord` — used for
  best-effort abort reports. `renderJUnit`/`renderJSON` render to disk via
  `Write(spec, report)`; `sanitizeXML` strips XML-1.0-illegal control
  chars/BMP noncharacters (invalid UTF-8 bytes are handled fine — they
  become `U+FFFD` via `strings.Map`, not a bug).
- `internal/controller.go` `Run()` collects a `[]report.TestRecord` as it
  runs tests, and only calls `writeAbortReports`/`writeReports` from three
  specific sites (skipErr, stopOnFail, test runErr) — every other early
  `return err` in `Run()` (the `--until` exit paths, teardown-step
  failure, node-teardown failure, platform-teardown failure) skips report
  writing entirely, discarding already-collected `records`. See
  [[feedback-dart-ci-integration-review]] finding 1.
- `cmd/dart/main.go` `runCheck()` is a config-only validator: it builds
  `mocks[node.Name] = &checkNode{...}` for every node **unconditionally**,
  bypassing `nodetypes.CreateNodesWithWrappers` (where `type:` is
  switched on/validated) entirely, and never touches `*cfgFlags.Report`
  or `*cfgFlags.Until`. So `--check` is a false-negative for bad node
  `type:`, bad `--report` specs, and bad `--until` targets — only
  YAML-shape errors (from `config.LoadConfiguration`, e.g. duplicate node
  names) and step/test option-wiring errors (from `CreateSteps`/
  `CreateTests`) are actually caught. See
  [[feedback-dart-ci-integration-review]] finding 3.
- `internal/formatters/logwriter.go` — `cleanLogWriter` backs `--log`,
  collapsing `\r`-redraws to final state and stripping ANSI via `ansiRe`.
  The regex doesn't match CSI private-mode sequences (`\x1b[?` prefix,
  e.g. cursor hide/show `\x1b[?25l`/`\x1b[?25h` that yacspin emits) — see
  finding 4. Also: `internal/stream/coordinator.go`'s debug-line writers
  hardcode `os.Stdout`/`os.Stderr` and bypass whatever writer the
  formatter was constructed with, so `--debug` streamed output never
  reaches a `--log` file — pre-existing code, but a real gap in the new
  feature's "clean transcript" promise. See finding 5.
- `--iterations N` in `RegisterHooks` (main.go) reruns `Ctrl.Run()` N
  times; each run's report write is a plain `os.WriteFile` to the same
  configured path, so only the *last* iteration's results survive on
  disk even though the process exit code correctly reflects a failure
  from *any* iteration (`lastErr` is sticky). See finding 2.

Full detail and verification method in
[[feedback-dart-ci-integration-review]].
