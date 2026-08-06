# Review: test evaluation (`internal/eval` + consumers)

- **Scope:** `internal/eval/`, `pkg/testtypes/execute.go`, `pkg/testtypes/base.go`,
  result consumption in `internal/controller.go`, `ExecutionResult` in
  `internal/execution/`.
- **Started:** 2026-08-06
- **Status:** complete (2026-08-06)

## Bugs

- [x] **Shared one-shot stdout reader across evaluators.**
  `ExecutionResult.Stdout` is a plain `io.Reader`; both `EvaluateMatch` and
  `EvaluateContains` call `io.ReadAll` on it. With more than one output-based
  check configured, the first evaluator drains the stream and the rest see an
  empty string — and because evaluators run in map-iteration order, which one
  wins is random per run.
  *Fix:* cached `StdoutBytes()`/`StderrBytes()` accessors on `ExecutionResult`
  that read the underlying stream once; evaluators use the accessors.

- [x] **Unchecked type assertions panic on bad config.**
  `match`/`contains` construction used `v.(string)`; a non-string YAML value
  (`match: 123`) crashed DART instead of producing a config error.
  *Fix:* validated parsing with comma-ok assertions in evaluator factories.

- [x] **`EvaluateResult.Err` silently swallowed.**
  The controller only inspected `Passed`/`Details`; an evaluator error
  (`Err` set, `Details` nil) printed a failed check with no explanation.
  *Fix:* controller prints the evaluation error when `Err` is set.

- [x] **Unstable sort on test `Order`.**
  `sort.Slice` in `CreateTests` can reorder tests that share an `Order` value
  (including the default zero) between runs.
  *Fix:* `sort.SliceStable` preserves config-file order for ties.

- [x] **Silent float truncation for `exit_code`.**
  `exit_code: 1.7` became `1` via `int(v)` with no error.
  *Fix:* factories reject non-integral numbers.

- [x] **Stderr never drained.**
  Nothing read `Stderr`, risking pipe-buffer blocking on chatty commands and
  discarding diagnostic output.
  *Fix:* stderr is drained after test execution via the cached accessor and is
  now assertable (see Gaps).

- [x] **Teardown failure drops test results.**
  A post-execute (teardown) failure returned early from `ExecutionTest.Run`,
  discarding the already-available test outcome.
  *Fix:* evaluations run and results are returned alongside the teardown
  error; the controller records/prints results before acting on the error.

## Improvements

- [x] **Evaluator registry instead of a switch in `NewExecuteTest`.**
  Construction/validation of evaluators moves into `internal/eval` behind a
  `name → factory` registry, so new evaluation types don't touch test-type
  code and future test types can reuse the parsing.

- [x] **Drop pointer-to-map for `BaseTest.evaluations`.**
  Maps are reference types; the extra indirection serves nothing.

- [x] **Simplify pass/fail collection in `ExecutionTest.Run`.**
  `if result.Passed == true { append(true) } else { append(false) }` becomes
  `append(passed, result.Passed)`; evaluators run in sorted-name order so
  output is deterministic.

- [x] **`match` trim behavior configurable.**
  `Trim` was hardcoded `true` and trimmed only `"\n\r "` (no tabs).
  *Fix:* `match` accepts either a plain string (trim defaults on, now
  whitespace-aware) or `{value: ..., trim: false}` for exact matching.

- [x] **Empty `evaluate:` blocks.**
  A test with no checks passes vacuously. The controller already reports these
  distinctly as "ran" rather than "passed", which is intentional for
  run-only tests — documented here so the decision isn't revisited.
  Status: **won't fix** (behavior is by design; the "ran" state is the signal).

## Gaps (missing evaluation types)

- [x] `regex` / `stderr_regex` — pattern match on output (compiled and
  validated at config-load time).
- [x] `not_contains` — negated substring check.
- [x] `exit_code` list form (`[0, 1]`) and `exit_code_not` — multi-value and
  negated exit-code contracts.
- [x] `stderr_contains` / `stderr_match` / `stderr_empty` — stderr was
  previously invisible to assertions.
- [x] `empty` — assert stdout is empty (whitespace-insensitive).
- [x] `line_count` — assert number of output lines.
- [x] `gt` / `lt` / `ge` / `le` — numeric comparison of stdout.
- [x] `max_duration` — assert the test command completed within N seconds
  (requires `Duration` on `ExecutionResult`, recorded around `Execute`).
- [x] `json_path` — assert a field in JSON output via dot-path
  (`{path: "a.b[0].c", equals: value}`), no external dependency.

## Resolution log

- **2026-08-06** — All findings resolved in one pass on branch
  `improve/eval-review`:
  - `internal/eval` restructured around a `name → factory` registry
    (`eval.New` / `eval.Parse`); all option parsing validates types and
    reports config-time errors instead of panicking.
  - `ExecutionResult` gained cached `StdoutBytes()`/`StderrBytes()` accessors
    and a `Duration` field (recorded around the test command in
    `ExecutionTest.Run`).
  - New evaluation types: `exit_code` list form, `exit_code_not`, `regex`,
    `stderr_regex`, `not_contains`, `stderr_contains`, `stderr_match`,
    `empty`, `stderr_empty`, `line_count`, `gt`/`lt`/`ge`/`le`,
    `max_duration`, `json_path`.
  - Controller now surfaces evaluation errors (`EvaluateResult.Err`), prints
    checks in sorted order, and records results delivered alongside a
    teardown error before aborting.
  - Unit tests added: `internal/eval/evaluate_test.go` (including a
    regression test for the shared-reader bug),
    `internal/execution/execution_test.go`.
  - README gained a "Test Evaluation Reference" section documenting every
    check.
  - Incidental fix: the node-cleanup error path in `internal/controller.go`
    built its message with `fmt.Sprintf` but never printed it.
