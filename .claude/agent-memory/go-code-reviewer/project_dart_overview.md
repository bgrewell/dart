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
