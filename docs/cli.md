# Command Line Reference

Flags, exit codes, and CI integration for the `dart` command.

[← Back to the README](../README.md)

### Options

```bash
Usage: dart [OPTIONS] [ARGUMENTS]

Version: dev
Date: dev
Codebase: dev (dev)

Description: DART is a distributed systems testing framework
  designed to make it easy to perform automation and
  integration testing on a wide variety of distributed
  systems.

Options:
  Default: Default Options
    -c        --config          config.yaml  The path to the configuration file
    -v        --verbose         false        Enable verbose output
    -d        --debug           false        Enable real-time streaming of command output
    -p        --pause-on-error  false        Pause on error
    -s        --stop-on-error   false        Stop on error
    -setup    --setup-only      false        Only run the setup steps
    -teardown --teardown-only   false        Only run the teardown steps
    -i        --iterations      1            Number of iterations to run
    -u        --until           -            Run up to and including this step or test, then stop
    -ub       --until-behavior  exit         Behavior when --until target is reached: exit (default) or pause
    -r        --report          -            Write machine-readable results: format:path (junit:results.xml, json:results.json; comma-separate for both)
    -V        --version         false        Print version information and exit
    -ck       --check           false        Validate the configuration and print the plan without running anything
    -l        --log             -            Write a clean (color-free) transcript of the run to this file
    -var      --vars            -            Override suite variables: key=value[,key=value...]
    -o        --only            -            Run only tests carrying one of these tags: tag=name[,name...]
    -sk       --skip            -            Exclude tests carrying any of these tags: tag=name[,name...]
```

Flags with no default show `-` in the default column; that is the placeholder
the usage library prints for an unset string, not a literal value.

Several short forms do not follow from their long names and are worth
memorising: `-ck` for `--check`, `-ub` for `--until-behavior`, `-var` for
`--vars`, `-sk` for `--skip`, and `-V` (capital) for `--version`.

DART accepts no positional arguments. The suite file is selected with
`-c`/`--config`, which defaults to `config.yaml` in the working directory.

Warning: `dart --help` still prints `[ARGUMENTS]` in its synopsis — that string
is hardcoded by the underlying usage library — but a positional argument is not
parsed. `dart suite.yaml` aborts with a runtime panic and exit code 2 instead of
running the suite. The suite file always goes after `-c`: `dart -c suite.yaml`.

### Flag Syntax

Flags are parsed by Go's standard `flag` package, which imposes a few rules
the option table does not show:

- **One dash or two makes no difference.** `-config`, `--config`, `-c`, and
  `--c` all name the same flag. Short and long names are registered
  independently, so either name accepts either dash count. The mix of `-c` and
  `--check` styles in the examples below is presentation, not syntax.
- **Boolean flags are set by presence.** `-v`, `--verbose`, `--check`, `-s`,
  and the other on/off flags take no value word. An explicit value must be
  attached with `=`: `--verbose=false`. Writing `--verbose false` is not
  supported, because the `false` is read as a positional argument, which `dart`
  does not accept.
- **Value flags take either form.** `--config suite.yaml` and
  `--config=suite.yaml` are equivalent, as are `-i 3` / `-i=3` and
  `--vars key=value` / `--vars=key=value`.
- **Flags must precede any non-flag token.** Parsing stops at the first
  argument that is not a flag. Since `dart` takes no positional arguments, a
  stray token — including a value word after a boolean flag — is an error.

### Flag Validation

Two flag values are checked before any configuration is loaded. Both print a
message on stderr and exit 1:

- `-i`/`--iterations` must be at least 1. `dart -i 0` fails with
  `Error: iterations must be at least 1 (got 0)` rather than exiting green
  having run nothing.
- `-ub`/`--until-behavior` must be exactly `exit` or `pause`. Any other value
  fails with `Error: until-behavior must be "exit" or "pause" (got "…")`
  rather than silently falling back to `exit`.

Report specs (`-r`) and tag filters (`-o`, `-sk`) are validated as the run
starts, and `--check` validates them without touching infrastructure.

### Verbose Output

`-v`/`--verbose` changes only the test-run output. Without it the console shows
failing checks and errors; with it every *passing* evaluator check is also
printed with the value it observed — the exit code, the matched output, the
measured duration. Expected-versus-actual detail appears only for failing
checks. Every skipped test prints the reason its condition triggered
(`skip_if condition met: <command>` or `skip_unless condition not met:
<command>`).

A non-verbose run shows a skipped test as a bare `skipped` marker. The reason
is still recorded in `--report json` (the `reason` field) and in
`--report junit` (the `<skipped message="…">` attribute). Setup and teardown
step output is unaffected by `-v`; real-time command output is a separate flag,
`-d`/`--debug`.

### Pausing on Error

`-p`/`--pause-on-error` behaves differently depending on which phase fails, and
it always reads from stdin.

**Setup phases** — platform setup, node setup, and `setup:` steps — present an
interactive menu:

```text
Setup step '<name>' failed. Options:
  [c]ontinue - Skip and continue with setup/tests
  [r]etry    - Retry this step
  [q]uit     - Cleanup and exit
Choice [c/r/q]:
```

`c` or `continue` skips the failed item and proceeds; `r` or `retry` re-runs
the same item and may be repeated indefinitely; `q` or `quit` aborts and runs
cleanup. Any other input — including a bare Enter or end-of-file on stdin — is
treated as quit, so the run aborts by default.

**The test phase** offers no choices. A failing test prints
`Press enter to continue` and resumes with the next test once a line is read.
The prompt fires once per failed test rather than once per failed check: every
check result is printed first, then a single pause on the test's overall
outcome. A test that returns an error rather than failing a check shows the
same prompt, but the suite aborts immediately afterwards regardless of the
input.

**Teardown is unaffected.** `--pause-on-error` never prompts during teardown
steps or node and platform teardown.

Warning: because the flag reads stdin directly, it suits interactive debugging
only. In CI a held-open stdin hangs the run indefinitely, and an
end-of-file stdin sends the setup menu to its abort default — so a failure the
flag was meant to make recoverable instead ends the run. `--stop-on-error` is
the fail-fast flag for pipelines.

### Setup-Only and Teardown-Only

`--setup-only` (`-setup`) and `--teardown-only` (`-teardown`) split a normal run
into two halves so an environment can be built, inspected by hand, and then
removed.

`--setup-only` runs platform setup, node setup, and every setup step, then
exits 0 immediately. It performs no cleanup of any kind: no teardown steps, no
node teardown, no platform teardown. Docker containers and LXD/Incus instances
are deliberately left running so they can be inspected, and nothing on an SSH
node's remote host is touched — `Teardown` is a no-op there. The only thing
that happens at process exit is the release of client-side handles: for Docker
and LXD nodes that destroys nothing, and for SSH nodes it closes the connection
and any bastion tunnel, leaving the remote host itself untouched.

`--teardown-only` is the counterpart that removes what `--setup-only` left
behind. It skips platform setup, node setup, setup steps, and all tests, then
runs, in order:

1. the `teardown:` steps, best-effort — a failing step is reported and the
   sequence continues;
2. node teardown, in the order the nodes are declared in the config file;
3. `Teardown` on each configured platform, in reverse declaration order.

Note: when both flags are passed, `--teardown-only` wins. It is evaluated
first and the setup half never runs.

Warning: because `--setup-only` never cleans up, a run that is not followed by
a `--teardown-only` run leaves infrastructure allocated. Re-running
`--setup-only` against the same config does **not** tolerate what the first run
left behind: node setup fails on the name already in use (a Docker container
name conflict surfaces as `could not create container: ...`), because only the
teardown paths tolerate a missing or existing resource. The environment must be
removed with `--teardown-only`, or by hand, before setup can run again.

```bash
dart -c suite.yaml --setup-only      # build the environment, leave it running
docker exec -it mynode bash          # inspect it by hand
dart -c suite.yaml                   # optionally run the full suite separately
dart -c suite.yaml --teardown-only   # remove steps, then nodes, then platforms
```

### Stopping Early

```bash
dart -c suite.yaml -u "install locker"        # stop after that setup step
dart -c suite.yaml -u "lock system as jim"    # stop after that test
dart -c suite.yaml -u 4                       # stop after the 4th test that runs
dart -c suite.yaml -u 4 -ub pause             # stop, wait for enter, then finish
```

`-u`/`--until <target>` runs up to and including the named point, then applies
`-ub`/`--until-behavior`. The target is matched, in order, against:

- a setup step `name`,
- a test `name`,
- a 1-based test number — the number printed beside each test in the
  `Running tests` output. Numbering covers the tests left after `--only`/`--skip`
  filtering — including tests later skipped by `skip_if`/`skip_unless`, which
  keep their number — so only tag filters renumber the list.

The target is validated before any platform or node setup happens. An unknown
target aborts immediately with an error listing every available setup step and
numbered test, and DART exits 1 without creating anything. A target that exists
in the suite but was removed by `--only`/`--skip` reports that specific reason
instead.

`-ub`/`--until-behavior` accepts only `exit` (the default) or `pause`:

- `exit` stops the run at that point and exits 0. Like `--setup-only`, this is
  a deliberate early return: teardown steps, node teardown, and platform
  teardown are all skipped, so containers, networks, and node-side state stay
  up for inspection. `--teardown-only` removes them afterwards. Note: with
  `-i N`, every iteration still runs and stops at the same point.

  Warning: that exit 0 is unconditional — the failure count is never consulted
  on this path, so a `--until` run exits 0 even when the tests that did run
  failed. `--until` is an inspection aid, not a pass/fail gate; a pipeline that
  needs the verdict must assert on the report file, which records the real
  counts.
- `pause` prints `Reached --until target "<target>". Press enter to continue
  execution...`, waits on stdin, then resumes the run normally — remaining
  setup steps or tests, teardown steps, and node and platform teardown all
  proceed. Reading stdin makes this unsuitable for non-interactive CI.

### Iterations

`-i N` (`--iterations N`) repeats the entire run lifecycle N times: platform
setup, node setup, setup steps, tests, teardown steps, node teardown, and
platform teardown all happen once per iteration, so containers, networks, and
other per-run resources are created and destroyed each time.

A failing iteration does not stop the loop. The failure is remembered and the
remaining iterations still run, including when `--stop-on-error` aborts an
individual iteration early. The process exits 1 if any iteration failed and 0
only if all of them passed; the error printed at the end is the last failure
seen.

Report paths are suffixed with the iteration number only when N is greater
than 1 — `results.xml` becomes `results-1.xml`, `results-2.xml`, and so on —
so a passing final iteration cannot mask an earlier failure. Suffixing applies
to abort-path reports as well. With the default `-i 1` the configured path is
written unchanged.

### What an Abort Skips

The `teardown:` steps in a suite run only when the suite reaches the end of its
test list. Any error that ends the run early returns before that phase:

- a failing test under `-s`/`--stop-on-error`,
- a test that errors out rather than merely failing an evaluation (this one
  aborts with or without `-s`),
- an error raised while evaluating a test's skip condition,
- a platform, node, or setup-step failure, unless `-p`/`--pause-on-error` is
  used to retry or continue past it.

On those paths DART prints `[+] cleaning up after error` and performs node
teardown and platform teardown only, for the nodes and platforms that finished
setup, platforms in reverse order. User-defined `teardown:` steps do not run.
`--setup-only` and `--until` with the default `exit` behaviour go further: they
skip teardown steps *and* node and platform teardown, leaving the environment
up for inspection.

Cleanup that must happen on every path therefore belongs in node or platform
teardown, where container and project deletion is handled automatically. A
second pass in a pipeline's always-run block covers the rest:

```bash
dart -c suite.yaml -s || true
dart -c suite.yaml --teardown-only
```

### CI Integration

```bash
dart -c suite.yaml -r junit:results.xml,json:results.json   # test panels + tooling
dart -c suite.yaml -l run.log                               # transcript with colors stripped
dart -c suite.yaml --check                                  # validate config + print plan, run nothing
```

Note: `--log` always strips colors and escape sequences, but spinner redraws
collapse to their final state only when output is a terminal. Redirected — the
CI case — each spinner frame arrives on its own line and the log keeps them all.

JUnit output feeds GitHub/GitLab/Jenkins test panels, with skips and failure
details included, and both formats carry per-test and total durations. JSON
adds the `ran` count and a machine-readable per-test `status` (`pass`, `fail`,
`skip`, `ran`, `error`) for custom tooling — JUnit has no element for `ran`, so
a test with no `evaluate:` block is written there as a bare passing testcase.

`--check` validates node types, report specs, tag filters, and the full option
set of every step and test against mock nodes. It is a pre-commit or CI lint
that touches no infrastructure; node connectivity is not exercised.

Once the test phase begins, a report is written on every exit: test failures,
`-s`/`--stop-on-error`, an error inside a test, a `--until` stop on a test, and
teardown-step, node-teardown, or platform-teardown failures. Nothing is written
if the run aborts before the first test — platform setup, node setup, fact
gathering, step and test construction, a failing setup step, or `--until`
pointing at a setup step. `--setup-only`, `--teardown-only`, and `--check`
never write reports either. A missing report file is therefore an early-abort
signal rather than "no results"; the exit code and the `--log` transcript say
which. Note that `--stop-on-error` applies only to the test phase: a failing
setup step aborts unconditionally unless `--pause-on-error` intervenes.

Note: an abort report lists only the tests that actually executed. Tests after
the abort point are omitted from the file rather than recorded as skipped, and
the JUnit `tests=` count and the JSON totals derive from that shortened list.
The `skip` status is reserved for tests whose `skip_if`/`skip_unless` condition
matched. A CI panel therefore shows a lower, run-to-run-varying test count on
aborted runs, so a fixed expected-test-count assertion does not survive
`-s`/`--until`; a stable total requires running without `--stop-on-error`.

`--log` captures debug-streamed command output too, so `-d` and `-l` combine
into a full transcript.

Warning: `--teardown-only` is best-effort and always exits 0 — see
[Exit Codes](#exit-codes) for what that hides and how to detect it.

### Report Formats

`--report` (`-r`) takes one or more `format:path` specs, comma-separated:

```bash
dart -c suite.yaml -r junit:results.xml,json:results.json
```

- **Formats**: `junit` and `json` only. Any other format is rejected before the
  suite runs: `Error: unknown report format "tap" (supported: junit, json)`,
  exit 1.
- **Grammar**: both halves are required. A value with no `:` or with an empty
  path is rejected: `Error: report spec "junit" must be format:path (e.g.
  junit:results.xml)`, exit 1.
- **Paths** resolve relative to the working directory the `dart` process was
  started in, not the directory holding the config file. An existing file at
  that path is overwritten, and files are created with mode `0644`.
- **The parent directory must already exist.** DART does not create it.
  `-r junit:reports/results.xml` with no `reports/` directory fails with
  `open reports/results.xml: no such file or directory`, so a pipeline needs a
  preceding `mkdir -p reports`.
- **A report write failure fails the run.** On a completed suite an unwritable
  report path turns an all-passing run into exit 1 with
  `Error: writing junit report to <path>: …`, printed after the normal results
  summary. Reports written on an early abort are best-effort by contrast: a
  write failure there prints `Warning: …` and does not mask the original abort
  cause. A completed suite that cannot write its report prints both lines — the
  abort-path writer still runs on the way out and retries — but it is the
  `Error:` line that sets exit 1.

#### JSON schema

```json
{
  "suite": "string",
  "passed": 0,
  "failed": 0,
  "skipped": 0,
  "ran": 0,
  "duration_seconds": 0.0,
  "tests": [
    {
      "name": "string",
      "node": "string",
      "status": "pass",
      "duration_seconds": 0.0,
      "failures": ["check: detail"],
      "reason": "string"
    }
  ]
}
```

`failures` is omitted when empty, `reason` is omitted when empty, and `tests`
is `null` when no test records were produced. The status vocabulary is:

| Status | Meaning |
|--------|---------|
| `pass` | Every configured check passed. |
| `fail` | At least one check failed or errored during evaluation; the failing `check: detail` lines are in `failures`. |
| `skip` | A skip condition matched before the test ran; the condition is in `reason`. |
| `ran` | The test executed but had no evaluations configured, so there is nothing to pass or fail. |
| `error` | An infrastructure error: the test could not produce results (node unreachable, run error), or a skip condition itself failed to evaluate. The error text is in `failures`. |

Note: `failed` is the sum of the `fail` and `error` records, so
`passed + failed + skipped + ran` equals the length of `tests`.

Note: records for skipped tests, and for errors raised before the test body
ran, carry `duration_seconds: 0`.

#### JUnit schema

One `<testsuite>` per report:

```xml
<testsuite name="SUITE" tests="N" failures="F" errors="E" skipped="S" time="0.000">
  <testcase name="TEST NAME" classname="NODE NAME" time="0.000"/>
</testsuite>
```

Suite attributes: `name` is the suite name, `tests` is the number of test
records, and `time` is the suite duration in seconds to three decimals.
`errors` counts the `error` records, and `failures` is the total failure count
minus the errors — that is, the JUnit `failures` attribute counts only `fail`
records and excludes infrastructure errors. `skipped` counts the `skip`
records.

Per testcase, `name` is the test name, `classname` is the node the test
targeted, and `time` is the test duration in seconds to three decimals. A
`fail` emits `<failure message="checks failed">` whose body is the `failures`
lines joined by newlines; an `error` emits `<error message="infrastructure
error">` with the same body treatment; a `skip` emits
`<skipped message="SKIP REASON"/>` with no body. `pass` and `ran` both emit a
bare `<testcase/>` with no child element, so a `ran` test is indistinguishable
from a pass in a CI test panel.

Note: control characters that XML 1.0 disallows — everything below `0x20`
except tab, newline, and carriage return, plus U+FFFE and U+FFFF — are stripped
from the suite name, test names, classnames, and message bodies before
marshalling, so ANSI escapes in captured command output cannot produce files
that CI parsers reject.

### Exit Codes

- **0**: No test failed. Note: a run that executes no tests also exits 0.
  `--setup-only`, `--teardown-only`, `--check`, `--version`, and `--until` with
  the default `exit` behaviour all end this way, as does a suite that declares
  no tests at all.
- **1**: One or more tests failed, or an error ended the run — a configuration
  that failed to load or validate, a rejected flag value (`--iterations` below
  1, an invalid `--until-behavior`), a `--until` target that matches nothing, a
  tag filter that excluded every test, a platform, node, or setup-step failure,
  a teardown failure on a normal run, or a report that could not be written.
- **2**: A positional argument was passed. `dart` registers none, and the
  underlying usage library panics rather than reporting the mistake; the suite
  file belongs after `-c`.

Skipped tests (`skip_if`/`skip_unless`) are reported separately and never
affect the exit code.

A `--only`/`--skip` combination that excludes every test is an error, not a
green run: DART reports `the --only/--skip tag filter excluded every test;
nothing ran (check the tag names against the suite)` and exits 1, so a mistyped
tag cannot produce a permanently passing pipeline that tests nothing.

These exit codes allow DART to integrate with automated DevOps workflows,
ensuring that issues are immediately flagged during continuous integration and
deployment processes — subject to the two exceptions below.

**Warning: a malformed flag exits 0.** Flag parsing comes from
`github.com/bgrewell/usage`, whose `NewUsage` assigns `flag.Usage` to its own
`PrintUsage`, and `PrintUsage` ends in `os.Exit(0)`. Go's `flag` package calls
that usage function from `failf` before reaching its own `os.Exit(2)`, so any
parse error — an unrecognised flag such as `dart --bogusflag`, or a bad value
for a defined flag such as `dart -i abc` — prints the error to stderr, prints
the usage screen to stdout, and exits 0 having run nothing. `--help` also exits
0, so the exit code alone cannot distinguish help from a typo.

A green exit code is therefore not proof that the suite ran. Asserting on a
produced artifact is the reliable gate:

```bash
dart -c suite.yaml -r junit:results.xml
test -s results.xml || { echo "dart did not run (bad invocation?)"; exit 1; }
```

With `-i N` each iteration writes its own file (`results-1.xml`,
`results-2.xml`, …), so the check covers the expected count of files.

**Warning: `--teardown-only` is best-effort and always exits 0.** In
teardown-only mode DART attempts every teardown step, then every node teardown,
then every platform teardown, continuing past each failure so that one broken
step cannot strand the rest of the cleanup. Failures are reported on the
console — the task is marked `error` and the underlying error is printed, for
example `Error running teardown step "drop test network": command failed with
exit code 7` — but they are not propagated, and the command still exits 0. The
only teardown-only failure that exits non-zero is a malformed `teardown:`
section that cannot be constructed at all, such as an unknown step type, which
fails before any cleanup runs.

Note: this differs from a normal run, where a failing teardown step aborts the
remaining teardown and exits 1.

Note: no report is written in teardown-only mode. `-r junit:…` and `-r json:…`
are ignored on this path, so reports are not a substitute signal.

A CI cleanup job that must fail when cleanup fails has to scrape the console
stream, not the log file:

```bash
dart -c suite.yaml --teardown-only | tee teardown.log
if grep -qE '^Error (running teardown step|cleaning up)' teardown.log; then exit 1; fi
```

Warning: `--log` does not capture these messages. The three the teardown-only
path emits — `Error running teardown step %q: …`, `Error cleaning up node
%s: …`, and `Error cleaning up %s environment: …` — are printed directly to
stdout rather than through the formatter that `--log` wraps, so a `-l` file
contains the task lines and none of the errors. Piping the console output is
the only way to see them.

Note the `if` rather than a trailing `grep … && exit 1`: as the last line of a
script, a `grep` that matches nothing exits 1 and would fail the job precisely
when cleanup succeeded.

---

### Example Test Execution

Below is a simplified example of how DART logs its operations during a test run. The actual output includes color coding and more detailed formatting for clarity:

```bash
[+] Running test setup
  [ local       ] running setup ...................... done
  [ locker-test ] running setup ...................... done
  [ locker-test ] ensure sshpass is installed ........ done
  [ locker-test ] ensure dns is working .............. done
  [ locker-test ] install locker ..................... done
  [ locker-test ] create user bob .................... done
  [ locker-test ] create user jim .................... done
  [ locker-test ] create user tom .................... done
  [ locker-test ] ensure password login is allowed ... done
  [ locker-test ] restart ssh ........................ done

[+] Running tests
  00001: [ locker-test ] verify locker is installed .................. passed
  00002: [ local       ] ssh to locker-test as bob ................... passed
  00003: [ local       ] ssh to locker-test as jim ................... passed
  00004: [ locker-test ] lock system as jim .......................... passed
  00005: [ local       ] ssh to locker-test as disallowed user bob ... passed
  00006: [ local       ] ssh to locker-test as allowed user tom ...... passed
  00007: [ locker-test ] unlock system as jim ........................ passed
  00008: [ local       ] verify bob can again access the system ...... passed

[+] Running test teardown
  [ local       ] running teardown ................... done
  [ locker-test ] running teardown ................... done

[+] Results
  Pass: 00008
  Fail: 00000
  Time: 4.21s
```

Every task and test line carries a node column: `[ ` plus the node name padded
to the width of the longest node name, plus ` ]`. Task lines put the box first,
right after the two-space indent; test lines put it after the zero-padded test
number and colon. Platform setup and teardown lines have no node, so they print
a blank box of the same width and the columns stay aligned.

The results block always prints `Pass:` and `Fail:` as five zero-padded digits.
`Skip:` appears only when at least one test was skipped, `Ran:` only when at
least one test executed with no evaluations configured, and `Time:` is the
suite elapsed time rounded to 10 ms.
