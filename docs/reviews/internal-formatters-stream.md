# Review: output layer (`internal/formatters`, `internal/stream`)

- **Scope:** `internal/formatters/` (standard formatter, completers,
  mocks), `internal/stream/` (debug tee writer, spinner coordinator).
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **Raw ANSI escapes leaked into non-terminal output.**
  Node names were colored with a hardcoded 256-color escape helper that
  ignored TTY detection, so piped/captured output (CI logs, `| tee`)
  contained literal `\033[38;5;46m` sequences — visible in every captured
  run. *Fix:* node names use the (previously declared but unused)
  `nodeNameColor` from the color package, which disables itself for
  non-terminal writers; the raw-escape helper is deleted. Regression test
  asserts captured output contains no 256-color escapes.

- [x] **Unknown detail types printed nothing.**
  `PrintPass`/`PrintFail` type-switched on string/int (+ the two result
  structs) and silently dropped anything else — a check whose details were
  a float or custom type showed a bare header with no explanation.
  *Fix:* shared `printDetails` falls back to `fmt.Sprint`.

- [x] **Six-digit test IDs panicked the zero-padding**
  (`strings.Repeat("0", 5-len(id))` with a negative count). Clamped.

## Improvements

- [x] **Formatter output is now injectable.** All output (including
  spinner output via yacspin's `Writer`) goes through an `io.Writer`
  defaulting to stdout; `NewStandardFormatterWithWriter` enables the new
  test suite and captured runs. Previously the package was untestable —
  every print went straight to `os.Stdout`.

- [x] **Test suites added** for both packages (previously 0%):
  formatter print methods (headers, pass/fail details incl. structured
  expected/actual, skip, results counts and zero-suppression), padding
  edge cases, node-box formatting, completer terminal states
  (passed/failed/ran/skipped/done/error), tee-writer capture and
  prefixing, `StreamCopy`, and the coordinator's no-spinner path.
  Coverage: formatters 0% → 61%, stream 0% → 56%.

- [x] **Notes, kept as-is** (documented so they aren't re-litigated):
  - `Update()` on both completers is a no-op (the status strings
    "preparing"/"running"/"cleanup" are never displayed). Enabling it
    means spinner message churn; deferred until someone wants it.
  - The tee writer flushes debug lines per `Write` call, so a partial
    line at a write boundary renders as two lines in debug output.
    Cosmetic; buffering until newline would fix it.
  - `yacspin.New` errors are ignored — the config is fixed and valid, so
    it cannot fail in practice.

## Resolution log

- **2026-08-07** — All findings resolved on branch
  `review/formatters-stream`. Verified end-to-end that piped DART output
  no longer contains raw escape sequences.
