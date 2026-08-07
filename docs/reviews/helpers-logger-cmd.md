# Review: helpers, logger, CLI entry point

- **Scope:** `internal/helpers/`, `internal/logger/`, `cmd/dart/main.go`.
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **`-i 0` exited green having run nothing.** The iteration loop
  simply didn't execute and the process exited 0 — a false-positive for
  any CI invocation with a computed iteration count. Iterations are now
  validated (≥ 1) with a clear error.

- [x] **A typo'd `--until-behavior` silently meant "exit".** The value is
  now validated against exit/pause.

- [x] **Node close raced process exit.** `OnStop` launched
  `Ctrl.Close()` in a goroutine and returned immediately, so connection
  shutdown (e.g. SSH) could be cut off mid-flight; errors were also
  discarded. Close is synchronous now and failures are reported.

- [x] **All 17 `helpers.Err*` sentinel errors were dead code.** The
  registry refactors replaced every use with located config errors;
  the block is deleted (its `WrapError` file:line stamps pointed at the
  var-declaration site anyway, which was misleading). `WrapError`
  itself is widely used and stays.

- [x] **`internal/logger` was never gofmt-formatted** — the one file
  every format check had to exclude. Formatted; the repo is now clean
  under `gofmt -l` with no exceptions.

## Notes

- `--until` combined with `-i N`: an until-exit leaves nodes running by
  design, so a following iteration's setup can collide with the leftover
  state. Using both flags together is a user error today; a guard would
  be reasonable if it bites someone.
- The fx event logger only surfaces errors (level Error) — appropriate
  for a CLI; nothing to change.

## Resolution log

- **2026-08-07** — Findings resolved on branch
  `review/helpers-logger-cmd`. Tests added for `WrapError` caller
  location, `GetRandomId`, and `ShellQuote`; flag validation verified by
  running the binary with `-i 0` and a bad `-ub` (both exit 1 with clear
  messages).
