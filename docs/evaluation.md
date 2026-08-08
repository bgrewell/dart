# Test Evaluation Reference

Tests declare their pass/fail conditions in an `evaluate` block. Every listed
check must pass for the test to pass. Checks are evaluated and reported in
alphabetical order.

[← Back to the README](../README.md)

## Where the block goes

The `evaluate` block is a key inside a test's `options:` map —
`tests[].options.evaluate` — not a sibling of `name`, `node`, and `type`. It is
a map of check name to expected value:

```yaml
suite: Evaluation example
nodes:
  - name: localhost
    type: local
tests:
  - name: git is installed
    node: localhost
    type: execute
    options:              # evaluate lives here, under options
      command: git --version
      evaluate:
        exit_code: 0
        contains: "git version"
```

Three ways the block itself can go wrong:

- **Misplaced block.** An `evaluate:` placed at the test's top level (a sibling
  of `type:` rather than a key under `options:`) is silently discarded —
  `TestConfig` has no such field and config parsing is non-strict. The test then
  runs with zero checks, is reported as `ran` rather than passed, counts toward
  `Ran` instead of `Pass`/`Fail`, and the suite exits 0. There is no warning, so
  a misplaced block that would have failed the test produces a green run.
- **Unknown or misspelled check name.** Aborts the run with
  `Error: unknown evaluation type "<name>"` and exit status 1. Note: this
  happens during test construction, after platform setup, node setup, and fact
  gathering — nodes are created and then torn down ("cleaning up after error")
  before the message appears, so the failure surfaces later than a config syntax
  error, though no setup step or test ever runs.

  `file_hash` is the exception. Its `evaluate` block accepts only `md5`,
  `sha1`, and `sha256`; every other key — misspelled or not — reports
  `unknown hash algorithm "<name>" in test "<test name>" (supported: md5,
  sha1, sha256)`. That closure covers the whole vocabulary on this page, so
  `evaluate: {sha256: <digest>, contains: "x"}` is a configuration error
  rather than two checks.

  These names belong to one test type and are recognized only there; using one
  under a different type reports the same unknown-evaluation-type error:

  | Test type | Type-scoped check names |
  |---|---|
  | `http_request` | `status_code` |
  | `exists` | `exists` |
  | `ping` | `packet_loss`, `rtt_min`, `rtt_avg`, `rtt_max` |
  | `service_status`, `port_check` | `status` |
  | `consistency` | `all_equal`, `matching` |
  | `tls_cert` | `min_days_remaining`, `dns_names`, `issuer_contains`, `subject_contains`, `chain_valid` |
  | `file_hash` | `md5`, `sha1`, `sha256` |
- **Wrong value shape.** `evaluate` must be a map. A scalar or list aborts with
  `Error: evaluate must be a map in test "<test name>" (got <type>)` and exit
  status 1, at the same point in the run.

## Tests with no checks

A test that ends up with no checks is reported as "ran" rather than passed. In
practice this only happens for `type: execute`, which is the one type with no
default check — useful for steps that only capture or extract values. Every
other type either injects a default check or refuses to load:

| Type | Default check | When applied |
|------|---------------|--------------|
| `execute` | none | — (the only type that can report "ran") |
| `exists` | `exists: true` | always injected; a user-supplied `exists` value is honored, but the check cannot be removed |
| `service_status` | `status: active` | always injected; a user-supplied `status` is honored, but the check cannot be removed |
| `port_check` | `status: open` | always injected; a user-supplied `status` is honored, but the check cannot be removed |
| `file_content` | `readable` (exit code 0) | only when `evaluate` yields no checks |
| `http_request` | `status_code: 200` | only when `evaluate` yields no checks |
| `ping` | `packet_loss: 0` | only when `evaluate` yields no checks |
| `tls_cert` | `min_days_remaining: 0` (not expired) | only when `evaluate` yields no checks |
| `consistency` | `all_equal: true` | only when `evaluate` yields no checks |
| `reboot` | `rebooted` (exit code 0) | only when `evaluate` yields no checks |
| `file_hash` | none — config error | at least one of `md5`/`sha1`/`sha256` is required, and no other check on this page is accepted; an empty `evaluate` fails at test construction |

Note: omitting `evaluate` is a no-assertion no-op only for `execute`.

## Checks

### Exit code

| Check | Value | Passes when |
|-------|-------|-------------|
| `exit_code` | integer or list (`[0, 1]`) | Exit code equals the value (or is in the list) |
| `exit_code_not` | integer or list | Exit code is not the value (or not in the list) |

### Output content (stdout)

| Check | Value | Passes when |
|-------|-------|-------------|
| `match` | string or `{value: "...", trim: false}` | Output equals the value exactly; see the note on trimming below |
| `contains` | string | Output contains the substring |
| `not_contains` | string | Output does not contain the substring |
| `regex` | string | Output matches the regular expression anywhere in the stream (validated at config load) |
| `empty` | boolean | Output is empty / non-empty, ignoring whitespace |
| `line_count` | integer | Output has exactly N lines (trailing newlines ignored) |

`not_contains` and `line_count` inspect stdout only.

`match` and `stderr_match` accept either a plain string or a `{value, trim}`
map.

- `value` is required in the map form and must be a string; a missing `value`
  or any key other than `value`/`trim` is a configuration error.
- `trim` is optional, must be a boolean, and defaults to `true` in both forms —
  `match: {value: "ok"}` behaves identically to `match: "ok"`. Set
  `trim: false` for a byte-for-byte comparison.
- When `trim` is true, only the actual output is trimmed, and only of trailing
  `" \t\n\r"` characters. The expected value is never trimmed and leading
  whitespace is never removed, so `match: "  ok"` does not match output `ok`.

### Stderr (complete set)

| Check | Value | Passes when |
|-------|-------|-------------|
| `stderr_match` | string or `{value, trim}` map | Stderr equals the value; same trimming rules as `match` |
| `stderr_contains` | string | Stderr contains the substring |
| `stderr_regex` | string | Stderr matches the regular expression anywhere in the stream (validated at config load) |
| `stderr_empty` | boolean | Stderr is empty / non-empty, ignoring whitespace |

These four keys are the complete set of stderr checks. There is no
`stderr_not_contains`, `stderr_line_count`, `stderr_json_path`, or stderr
numeric bound; any such key is rejected at config load with
`unknown evaluation type`.

To assert against merged streams, redirect in the command itself
(`command: "mycmd 2>&1"`) and use the stdout checks.

Note: `regex` and `stderr_regex` perform an unanchored search — the pattern
only has to appear somewhere in the stream, so `regex: "ok"` passes against
output of `not ok`. Anchor with `^` and `$` for a whole-string match.

Warning: unlike `match`, the stream is not trimmed before matching. Because
most commands emit a trailing newline, `regex: "^ok$"` fails against output of
`ok\n` — Go's `$` matches only at end of text by default. The `(?m)` flag binds
`^` and `$` to line boundaries (`regex: "(?m)^ok$"`), and `(?s)` lets `.` match
newlines.

Note: patterns use Go's `regexp` package (RE2 syntax). Backreferences (`\1`)
and lookaround (`(?=`, `(?!`, `(?<=`) are unsupported and fail to compile.
Patterns are compiled while test objects are constructed, before any test runs,
so an invalid expression aborts the run up front rather than failing mid-suite.

### Numeric and structured output

| Check | Value | Passes when |
|-------|-------|-------------|
| `gt` / `ge` / `lt` / `le` | number | The whole of stdout, after trimming surrounding whitespace, parses as a single number and satisfies the comparison |
| `json_path` | `{path: "a.b[0].c", equals: value}` | The whole of stdout parses as a single JSON document and the value at the dot-path matches |

`gt` / `ge` / `lt` / `le` and `json_path` parse stdout only, regardless of what
the command writes to stderr.

Note: the numeric checks parse the entire trimmed stdout. Anything alongside
the number — a unit (`42ms`), a label (`count: 42`), a thousands separator
(`1,000`), or a second line of output — fails the check and is reported as
`Expected: numeric output > N`. Go's float syntax is accepted, so `1e3`, `+42`,
`0x1p4`, `Inf`, and `NaN` all parse. There is no field or line selection, and
no stderr equivalent (`stderr_gt` does not exist).

Rationale for the escape hatch: when the command emits anything besides the
bare number, an `execute` test can name the value with `extract:` using a
`{regex}` (capture group 1) or `{jsonpath}` spec and assert on it with a
comparator map — `gt`/`gte`/`lt`/`lte` (`ge`/`le` accepted), `eq`/`ne`, and
`within` with `tolerance_pct` or `tolerance`. A value that fails to parse is a
test failure, not a configuration or runtime error. `extract:` is read by the
`execute` test type only. See
[Value Extraction and Numeric Assertions](tests.md#value-extraction-and-numeric-assertions).

```yaml
# `gt: 0` would fail here: stdout is "p99=48.2us"
suite: Extraction example
nodes:
  - name: testbed
    type: local
tests:
  - name: p99 under budget
    node: testbed
    type: execute
    options:
      command: bench --summary
      extract:
        p99_us: { regex: "p99=([0-9.]+)us" }
      evaluate:
        p99_us: { lte: 49 }
```

Note: `json_path` parses the entire stdout as one JSON document. Only
surrounding whitespace is tolerated — a banner line, a trailing log line, or a
second document (JSON Lines) fails the check with `Expected: valid JSON
output`, even when the JSON itself is well-formed. The framework-generated
payloads (`tls_cert`, `consistency`) are always clean documents, so this
affects `execute` and `file_content` output and `http_request` response
bodies — whatever the command or the remote endpoint happens to produce. Remedies:
have the command write its logs to stderr (asserted separately via the
`stderr_*` checks), or use `extract: {name: {jsonpath: ...}}` with a
comparator, which decodes only the first JSON document and therefore tolerates
text after it. Warning: `extract` tolerates trailing text only — text before
the JSON fails both forms, and piping through `jq` does not help because `jq`
also aborts on trailing non-JSON.

Note: `json_path.path` is a dot-path — `status`, `result.items[0].name`. A
leading JSONPath-style `$.` or `$` is accepted and ignored, matching the
`extract: {jsonpath: ...}` extractor, so `path: "$.summary.throughput_mbps"`
and `path: "summary.throughput_mbps"` are equivalent and a path can be copied
between the two without editing.

`json_path` comparison is deliberately loose. If the value found at the path
and the `equals` value are both numbers, they are compared numerically, so
`equals: 200` matches JSON `200`, `200.0`, or `2e2` — int/float representation
does not matter. Otherwise both sides are rendered to their Go string form and
compared as strings, which has these consequences:

- Quoted and unquoted scalars are usually interchangeable: `equals: "healthy"`
  and `equals: healthy` behave the same, and `equals: "200"` matches JSON
  `200`. This is not universal — a quoted number only matches if the JSON
  number renders identically, and large or small magnitudes render in
  scientific notation (JSON `1000000` renders as `1e+06`), so unquoted numbers
  are preferable when asserting on numbers.
- Booleans compare by their text: `equals: true` and `equals: "true"` both
  match JSON `true`.
- JSON `null` renders as the literal string `<nil>`, so `equals: null` matches
  a JSON null (as does `equals: "<nil>"`).
- `equals` is intended for scalars. Objects and arrays are compared by their Go
  printed form (`map[a:x b:1]`, `[1 x true]`); the rendering is deterministic
  (map keys are sorted) but is Go syntax rather than YAML, so it is impractical
  to write. A deeper path that reaches a leaf value is the usable form.

Note: the same rendering is used in failure output, so a mismatch report shows
the Go string form of both sides.

### Timing

| Check | Value | Passes when |
|-------|-------|-------------|
| `max_duration` | seconds (fractional allowed) | The test command completed within the bound |

Note: `max_duration` is checked after the command returns — it never interrupts
or kills a slow command. The test's `timeout:` option is what stops waiting on
a hung command: it ends the suite's wait and fails the test with a `timeout`
check, while the command itself may keep running on the node (see
[Timeouts and Retries](tests.md#timeouts-and-retries)).

Note: the measured duration covers a single attempt. With `retry:` configured,
each attempt is timed independently, so `max_duration` bounds one attempt
rather than the test's total elapsed time — a test with `max_duration: 2.5` can
pass on a later attempt after minutes of retrying. A failing `max_duration`
check also counts as a retryable failure and triggers another attempt.

## Config-time validation

Evaluator values are checked when test objects are constructed, not when the
test runs — after platform and node setup, but before any setup step or test
executes. On a docker or lxd suite that means images are pulled and containers
created, then torn down again, before a typo in `evaluate` is reported.
`dart -c suite.yaml --check` catches every case below without touching
infrastructure.

| Check | Constraint |
|-------|------------|
| `exit_code` / `exit_code_not` | A scalar integer, or a non-empty list in which every element is an integer — an empty list and any non-integer element are both rejected |
| `line_count` | An integer >= 0 (a non-integral number such as `2.5` is also rejected) |
| `max_duration` | A number strictly greater than 0 — `0` and negatives are rejected |
| `json_path` | A map with both `path` and `equals`; any other key is rejected, and `path` is parsed at load time. There is no existence-only form: `json_path: {path: status}` is a config error, not a check that the path resolves |
| `match` / `stderr_match` (map form) | Requires `value` (string); `trim` is optional and must be a boolean; any other key is rejected |
| `regex` / `stderr_regex` | The pattern is compiled at load, so an invalid expression is a config error |
| `empty` / `stderr_empty` | Must be a boolean |
| `gt` / `ge` / `lt` / `le` | Must be a number |

## When a check errors

A check that cannot read the command's stdout or stderr reports an error
instead of a comparison. It prints as

```
-contains:
  evaluation error: <cause>
```

with no `Expected:`/`Actual:` pair, and appears in the report as
`<check>: evaluation error: <cause>`. An errored check counts as a failed
check, so the test's status is `fail` like any other failure — the only
difference is that the cause is a read failure rather than a mismatch. Unlike a
passing check, an errored one is printed whether or not `-v` is set.

Only checks that inspect output can error: `match`, `contains`,
`not_contains`, `regex`, `empty`, `line_count`, `gt`/`ge`/`lt`/`le`,
`json_path`, and the `stderr_*` variants. `exit_code`, `exit_code_not`, and
`max_duration` never read a stream and so never error. The composite test types
emit the same line when their own payload cannot be decoded — `consistency` and
`tls_cert` report `evaluation error:` for undecodable internal JSON — so the
outcome is framework-wide rather than specific to `execute`.

Note: output that is present but unusable is not an error. A non-numeric value
under `gt`, or non-JSON output under `json_path`, fails with a normal
expected/actual comparison.

Example combining several checks:

```yaml
suite: Health check
nodes:
  - name: localhost
    type: local
tests:
  - name: service reports healthy
    node: localhost
    type: execute
    options:
      command: "curl -s http://localhost:8080/health"
      evaluate:
        exit_code: 0
        json_path:
          path: status
          equals: healthy
        stderr_empty: true
        max_duration: 2.5
```

---
