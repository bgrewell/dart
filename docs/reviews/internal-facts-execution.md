# Review: facts + execution plumbing (`internal/facts`, `internal/execution`)

- **Scope:** `internal/facts/` (fact gathering, template rendering),
  `internal/execution/` (exec option parsing; the result accessors were
  covered by [internal-eval.md](internal-eval.md)).
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **Fact templating destroyed capture references.**
  `RenderTemplate` fed whole commands through text/template, and the
  runtime capture syntax (`{{capture.name}}`) is not valid template
  syntax — any suite using both facts and captures failed at the
  template-processing phase. *Fix:* capture references are shielded with
  placeholders before rendering and restored after, so they pass through
  untouched for the capture store to resolve at run time. Verified
  end-to-end with a suite using a fact template and a capture reference
  in the same command.

- [x] **`env:` exec option panicked on YAML input.**
  `OptionsToExecutionOptions` asserted `v.([]string)`, but YAML delivers
  lists as `[]interface{}` — configuring `env:` on a node crashed DART.
  All assertions in the option parser are now checked; wrong-typed
  options warn and are skipped rather than panicking, and both list
  shapes are accepted.

- [x] **Unset `sudo.env_var` silently produced an empty password.**
  Now warns that the variable is empty or unset.

- [x] **Fact commands ran in nondeterministic order** (map iteration) —
  now sorted by fact name for stable execution and output.

- [x] **Raw stream reads in fact gathering** replaced with the cached
  `StdoutBytes`/`StderrBytes` accessors, consistent with the rest of the
  codebase since the eval review.

## Notes

- Fact-gathering progress is displayed by the controller only after all
  facts are gathered (cosmetic; noted in the controller review).
- Fact values are rendered into commands verbatim — a fact value
  containing shell metacharacters is intentionally usable for
  composition, same trust model as the command itself.

## Resolution log

- **2026-08-07** — All findings resolved on branch
  `review/facts-execution`. New tests: capture-reference shielding
  (alone, mixed with fact calls, multiple refs), exec option coercion
  from YAML shapes, wrong-type no-panic paths, unknown-key handling.
  Coverage: facts 37% → 43%, execution 24% → 64%.
