# Test Evaluation Reference

Tests declare their pass/fail conditions in an `evaluate` block. Every listed check must pass for the test to pass; a test with no checks is reported as "ran" rather than passed. Checks are evaluated and reported in alphabetical order.

[← Back to the README](../README.md)

Tests declare their pass/fail conditions in an `evaluate` block. Every listed
check must pass for the test to pass; a test with no checks is reported as
"ran" rather than passed. Checks are evaluated and reported in alphabetical
order.

### Exit code

| Check | Value | Passes when |
|-------|-------|-------------|
| `exit_code` | integer or list (`[0, 1]`) | Exit code equals the value (or is in the list) |
| `exit_code_not` | integer or list | Exit code is not the value (or not in the list) |

### Output content (stdout)

| Check | Value | Passes when |
|-------|-------|-------------|
| `match` | string | Output equals the value exactly (trailing whitespace trimmed) |
| `match` | `{value: "...", trim: false}` | Output equals the value byte-for-byte |
| `contains` | string | Output contains the substring |
| `not_contains` | string | Output does not contain the substring |
| `regex` | string | Output matches the regular expression (validated at config load) |
| `empty` | boolean | Output is empty / non-empty, ignoring whitespace |
| `line_count` | integer | Output has exactly N lines (trailing newlines ignored) |

### Stderr

| Check | Value | Passes when |
|-------|-------|-------------|
| `stderr_match` | string or `{value, trim}` map | Stderr equals the value |
| `stderr_contains` | string | Stderr contains the substring |
| `stderr_regex` | string | Stderr matches the regular expression |
| `stderr_empty` | boolean | Stderr is empty / non-empty, ignoring whitespace |

### Numeric and structured output

| Check | Value | Passes when |
|-------|-------|-------------|
| `gt` / `ge` / `lt` / `le` | number | Output, parsed as a number, satisfies the comparison |
| `json_path` | `{path: "a.b[0].c", equals: value}` | Output parsed as JSON has the expected value at the dot-path |

### Timing

| Check | Value | Passes when |
|-------|-------|-------------|
| `max_duration` | seconds (fractional allowed) | The test command completed within the bound |

Example combining several checks:

```yaml
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
