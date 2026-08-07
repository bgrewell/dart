# Code review tracking

This directory tracks focused code-review passes over specific areas of the
codebase. Each document is scoped to one area (a package or a small cluster of
related packages) and records the findings from a review along with their
resolution status, so the work can be picked up, verified, or extended later
without re-deriving the analysis.

## Conventions

- One document per reviewed area, named after the area it covers
  (e.g. `internal-eval.md` for `internal/eval/` and its consumers).
- Findings are grouped into **Bugs**, **Improvements**, and **Gaps**
  (missing functionality surfaced by the review).
- Each finding gets a status checkbox and, once resolved, a short note on how
  it was resolved (and the commit where applicable).
- Findings that are consciously rejected are kept in the document and marked
  **won't fix** with the rationale, so the decision isn't re-litigated later.

## Documents

| Document | Area | Started |
|---|---|---|
| [internal-eval.md](internal-eval.md) | `internal/eval/`, `pkg/testtypes/`, result handling in `internal/controller.go` | 2026-08-06 |
| [pkg-steptypes.md](pkg-steptypes.md) | `pkg/steptypes/`, `MockNode` in `pkg/nodetypes/` | 2026-08-06 |
| [internal-controller.md](internal-controller.md) | `internal/controller.go`, `internal/errors.go` | 2026-08-07 |
| [internal-config.md](internal-config.md) | `internal/config/` | 2026-08-07 |
| [internal-formatters-stream.md](internal-formatters-stream.md) | `internal/formatters/`, `internal/stream/` | 2026-08-07 |
| [internal-facts-execution.md](internal-facts-execution.md) | `internal/facts/`, `internal/execution/` | 2026-08-07 |
| [pkg-nodetypes.md](pkg-nodetypes.md) | `pkg/nodetypes/` | 2026-08-07 |
