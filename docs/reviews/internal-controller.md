# Review: test controller (`internal/controller.go`)

- **Scope:** `internal/controller.go` (orchestration flow), `internal/errors.go`.
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **Test results keyed by name — duplicate names collide.**
  `testResults` was `map[testName]results`; two tests sharing a name
  overwrite each other in the summary counts. This is not an edge case:
  multi-node expansion (`node: [a, b]`) produces one test per node *with
  the same name*, so an expanded test was only ever counted once.
  *Fix:* results are collected in a slice, one entry per executed test.

- [x] **`--teardown-only` never ran the teardown steps.**
  The teardown-only path constructed the teardown step objects and then
  returned, leaving cleanup to the error-path defer — which only tears
  down nodes and platforms. The user's `teardown:` steps were silently
  skipped. *Fix:* teardown-only explicitly runs teardown steps
  (best-effort, reporting failures without aborting the remaining
  cleanup), then node teardown, then platform teardown.

- [x] **`stop-on-error` aborted mid-report.**
  A failing check returned from inside the per-check loop, so a test with
  several failing checks reported only the first, and `pause-on-error`
  prompted once per failing check instead of once per failing test.
  *Fix:* all check results for a test are printed first; stop/pause act
  once afterwards on the test's overall outcome.

- [x] **Nondeterministic node setup/teardown order.**
  Nodes were set up and torn down by ranging over the nodes map, so the
  order changed run to run — output jitter at best, breakage at worst
  when one node's setup depends on another's (shared networks). *Fix:*
  nodes follow config-file order everywhere (setup, teardown,
  teardown-only, error cleanup).

- [x] **Platform teardown errors swallowed in error cleanup.**
  The defer discarded `platform.Teardown()` errors without a trace.
  *Fix:* reported like node cleanup errors.

- [x] **`Close()` dropped node close errors.** Now aggregated and returned.

## Improvements

- [x] **Delete `internal/errors.go`.** A verbatim duplicate of
  `internal/helpers/err.go`; nothing referenced the `internal.Err*` names.

- [x] **Test suite for the controller** (previously 0% coverage): pass /
  fail / skip counting, duplicate-name counting, exit error on failure,
  stop-on-error and pause-free flows, `--until` validation and stop
  behavior, setup-only and teardown-only paths, node order determinism,
  and error-path cleanup of nodes and platforms — driven by a recording
  formatter fake, mock nodes, and a fake platform manager.

- [x] **Notes, kept as-is by design** (documented so they aren't
  re-litigated):
  - `--until` with `exit` behavior and `--setup-only` intentionally leave
    nodes/platforms running — that is their purpose (inspect state).
  - A teardown *step* failure aborts remaining teardown steps but the
    defer still tears down nodes/platforms; state is unknown at that
    point, so stopping is deliberate.
  - `stop-on-error` skips the user teardown steps (error path); node and
    platform cleanup still run via the defer.
  - Fact-gathering progress is displayed after gathering completes
    (cosmetic); making it live requires a facts API change — candidate
    for the facts review.

## Resolution log

- **2026-08-07** — All findings resolved on branch `review/controller`;
  controller behavior verified by the new `internal/controller_test.go`
  suite plus an end-to-end smoke run.
