---
name: feedback-dart-ci-integration-review
description: Verified bugs found reviewing DART's feature/ci-integration branch (internal/report, controller report-writing paths, --check, --log CleanLogWriter) — reusable when reviewing future CI-tooling changes in this repo.
metadata:
  type: feedback
---

Reviewed `feature/ci-integration` (diffed against `feature/timeouts-retry`) by
reading the diff, then verifying every hypothesis with either a throwaway
`internal/zzz_adversarial_*_test.go` repro (deleted after) or a live
`go run ./cmd/dart` invocation comparing `--check` vs a real run on a
scratch YAML (deleted after). See [[project-dart-overview]] for where the
report/log/check code lives. Confirmed findings:

1. **Report writing only covers three of the abort paths, not all of
   them.** `internal/controller.go` `Run()` calls
   `writeAbortReports`/`writeReports` only on: `skipErr`, `stopOnFail`
   test-failure, and test `runErr`. It does **not** write any report on:
   the `--until` exit path (line ~444 setup, ~584 tests), a teardown-*step*
   failure (~592-601), a node-teardown failure (~603-611), or a
   platform-teardown failure (~614-626) — all of which `return err`
   directly with no report call, even though `records` already holds a
   full set of completed test results at that point. Verified via repro:
   a suite where the one test passes but the teardown step/node teardown
   fails produces **zero** report file, discarding a fully-collected,
   fully-passing result set. This is the same class of bug the branch's
   first pass already fixed for `skipErr`/`stopOnFail`/`runErr` — the fix
   wasn't generalized to "always write on any abort," it was special-cased
   to just those three sites.

2. **`--iterations N` silently makes the report reflect only the last
   iteration, while the exit code reflects all iterations.**
   `cmd/dart/main.go` `RegisterHooks` loops `Ctrl.Run()` N times, and each
   `Run()` unconditionally `os.WriteFile`s to the same configured path.
   `lastErr` (used for the process exit code) is sticky across iterations,
   so the exit code correctly reflects "some iteration failed" — but the
   on-disk JUnit/JSON file is last-write-wins. Verified: iteration 1 fails
   (`failures="1"` on disk), iteration 2 passes and overwrites it
   (`failures="0"` on disk) — a CI job reads the exit code as failed but
   the JUnit panel as all-green. No accumulation, no per-iteration
   filenames, no warning.

3. **`--check` doesn't validate node `type:`, doesn't parse `--report`,
   doesn't validate `--until`.** `cmd/dart/main.go` `runCheck()` builds
   `mocks[node.Name] = &checkNode{...}` for **every** node regardless of
   `cfg.Nodes[i].Type` — it never calls
   `nodetypes.CreateNodesWithWrappers`, which is where `type:` gets
   switched on and validated. Verified via CLI: a YAML with
   `type: totally-bogus-type` prints "Configuration valid." / exit 0 under
   `--check`, then fails immediately with `unknown node type
   "totally-bogus-type"` on a real run. Also verified: `runCheck` never
   touches `*cfgFlags.Report`, so `dart --check -r bogusformat:x` also
   exits 0 while a real run fails on `ParseSpec`. (Duplicate node names
   *are* caught under `--check` — that validation lives in
   `config.LoadConfiguration` itself, shared by both paths, so it's not a
   gap.) The README's "`--check` performs full option validation for
   every node" claim is inaccurate as a result.

4. **`internal/formatters/logwriter.go`'s `ansiRe` regex
   (`\x1b\[[0-9;]*[A-Za-z]|\x1b\][^\x07]*\x07`) doesn't match CSI
   private-mode sequences** — anything with a `?` after `\x1b[`, e.g.
   `\x1b[?25l`/`\x1b[?25h` (cursor hide/show, which yacspin — the
   spinner library this codebase uses — emits on every spinner
   start/stop). Verified the regex fails to strip these in isolation.
   The existing `zzz_adversarial_logwriter_test.go` test named
   `TestAdversarial_CursorHideNotStripped` **passes**, but only by
   accident: in that test's exact byte sequence the escape happens to sit
   *before* the line's last `\r`, so the `\r`-collapse logic (which keeps
   only bytes after the last `\r`) discards it before the regex ever runs
   — not because the regex handles `?`-prefixed CSI codes. Constructing a
   sequence where the escape follows the last `\r`
   (`"spin1\rspin2\rdone\x1b[?25h\n"`) leaks `\x1b[?25h` straight into the
   "clean" log file. **Lesson: a passing adversarial test with the right
   *name* can still be testing the wrong *mechanism* — check the string
   position of the thing under test relative to other transforms in the
   pipeline (here, the `\r`-collapse ran before the regex and coincidentally
   ate the input), not just that the assertion passes.**

5. **New `--log` feature is silently incomplete under `-d/--debug`.**
   `internal/stream/coordinator.go`'s `WriteDebugLine`/
   `WriteDebugLineStderr` write hardcoded to `os.Stdout`/`os.Stderr`
   directly (pre-existing code, not touched by this branch), completely
   bypassing whatever writer `Formatter()` in `cmd/dart/main.go` wired up
   — including the `io.MultiWriter(os.Stdout, logWriter)` that backs
   `--log`. So real-time streamed command output (the highest-volume,
   most log-worthy content) never reaches the `--log` file. The feature
   works for the formatter's own headers/pass/fail lines but not for
   `--debug` streaming. Root cause predates this branch, but the new
   `--log` feature's "clean transcript of the run" promise doesn't hold
   for `--debug` runs, and nothing in the new code accounts for this
   pre-existing second output path.

Non-findings worth remembering (ruled out after investigation, don't
re-litigate):
- `sanitizeXML` handles invalid UTF-8 correctly — `strings.Map` turns each
  bad byte into a valid `U+FFFD` replacement rune, which passes the filter
  and is legal in XML 1.0. Verified with `\xff\xfe` input.
- The `writeAbortReports` calls in the `skipErr`/`stopOnFail`/`runErr`
  branches each unconditionally `return` right after, so there's no path
  where both `writeAbortReports` and the final `writeReports` fire for
  the same `Run()` call — no double-write.
- The `passed/failed/ran` ints used for the console summary and for
  `writeReports`'s `Report{}` are computed via a second, independent loop
  over `testResults` (not derived from `records`) — currently numerically
  consistent with `records`' per-test `Status` classification, but it's
  parallel logic that has to be kept in sync by hand on future edits
  (recommendation-level, not a live bug).
