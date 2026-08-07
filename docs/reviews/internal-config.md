# Review: configuration loading (`internal/config`)

- **Scope:** `internal/config/` (YAML loading, `!!load_from`, multi-node
  expansion, source-location tracking, error rendering).
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **Steps/tests with a missing or empty `node:` were silently dropped.**
  Expansion iterated the node list; zero nodes produced zero copies, so a
  test with a forgotten `node:` key simply vanished and the suite passed
  with fewer tests than written — the same false-positive class as skips
  rendering as passes. *Fix:* configuration validation rejects them with a
  located error. (The old behavior was even encoded in a test, updated.)

- [x] **Expanded tests shared their Setup/Teardown backing arrays.**
  Fact-template rendering rewrites those entries in place per node, so on
  a multi-node test the first node's rendered fact values leaked into its
  siblings' commands (templates are gone after the first render). *Fix:*
  expansion clones the slices per copy; regression test asserts sibling
  isolation. (`Options` maps were already safe — the facts processor
  builds new maps.)

- [x] **Malformed `!!load_from` directive panicked.** A missing closing
  parenthesis produced a negative slice index. *Fix:* a located error
  naming the line.

- [x] **Wrong error-snippet lines after `!!load_from`.** Inlining files
  shifts every subsequent line, but locations were recorded against the
  processed buffer and rendered from the original file — config errors
  pointed at the wrong lines. *Fix:* location extraction is skipped when
  load_from was used (no location beats a wrong one).

- [x] **No duplicate/unnamed node validation.** Duplicate node names
  silently collapsed in the controller's node map; unnamed nodes produced
  confusing downstream errors. Both are located config errors now.

- [x] **`path` used on OS paths.** `path.Dir`/`path.Join` and a `/`-prefix
  absolute check break Windows paths; switched to `filepath` equivalents.

## Notes / future

- Nested `!!load_from` (a loaded file containing another directive) is not
  processed — the directive survives into the YAML and fails to parse.
  Documented limitation; recursion would need cycle protection.
- Location matching is index-based against pre-expansion sequences, which
  is correct because extraction runs before expansion and expansion copies
  the locations.

## Resolution log

- **2026-08-07** — All findings resolved on branch `review/config`. New
  tests cover validation (duplicates, unnamed, missing node refs),
  expansion slice isolation, malformed and working `!!load_from`
  (including location skipping), and location extraction. Package
  coverage 23% → 61%.
