---
name: feedback-dart-retry-timeout-review
description: Verified bugs found reviewing DART's feature/timeouts-retry branch (retry, ExecuteWithTimeout, wait_for) — reusable when reviewing future retry/polling code in this repo.
metadata:
  type: feedback
---

Reviewed `feature/timeouts-retry` in DART by writing small throwaway
reproduction tests (added as `zzz_adversarial_*_test.go` in the target
package, run with `go test -race -run ...`, then deleted — never left in the
tree) instead of just reading the diff. This caught several bugs that pure
code-reading would have flagged only as "maybe":

1. **Generic retry applied to every test type with no opt-out.**
   `CreateTests` in `pkg/testtypes/base.go` attaches `retryTimeout`/
   `retryInterval` to `BaseTest` for any `cfg.Retry != nil`, regardless of
   `cfg.Type`. Since `RebootTest.Run` goes through the same `runProducer`,
   a `retry:` block on a `type: reboot` test with a failing evaluator causes
   the target to be **physically rebooted again on every retry attempt**
   (verified: 6 reboots in a 300ms retry window). No validation rejects
   this combination, and the README explicitly advertises retry as working
   "on every test type." **Lesson: when a generic cross-cutting knob (retry,
   timeout, etc.) is wired in one place for "every test type," check each
   concrete test type for side effects that are unsafe to repeat, not just
   whether the type compiles against the generic path.**

2. **`ExecuteWithTimeout`'s abandoned goroutine is invisible but not inert.**
   On timeout, the spawned goroutine keeps running `node.Execute(...)` and
   its result is discarded (buffered chan of 1, nobody reads it after the
   `select` fires). This means captures are *not* corrupted by stale
   results (good — discarding happens before capture code ever sees it),
   but the abandoned goroutine still: (a) leaks permanently if the
   underlying call never returns (verified: 5 leaked goroutines from one
   `retry:`+`timeout:` test run against a node that never returns), and
   (b) keeps calling into shared node/mock state concurrently with the
   *next* retry attempt, which can corrupt logical invariants even without
   a data race — verified against `MockNode.QueueResponse`'s FIFO queue:
   responses meant for "attempt N" get silently eaten by an abandoned
   "attempt N-1" goroutine racing on the same mutex, so the retry loop
   never observes the real command output and fails with a generic timeout
   error instead. **Lesson: "goroutine leak" isn't the only failure mode of
   an abandoned goroutine — check what shared, mutex-protected state it
   still touches after being "abandoned."**

3. **`wait_for`'s per-poll timeout is `remaining time to deadline`, not a
   fixed per-attempt slice.** (`pkg/steptypes/wait_for.go`, `Run` method.)
   This has two verified failure modes: if the command's natural latency is
   comparable to the *overall* `timeout`, the **first** poll alone consumes
   nearly the whole budget and the configured `interval` never gets a
   chance to produce multiple polls (verified: interval=20ms over a 300ms
   window produced exactly 1 real attempt, not ~15). Conversely, near the
   deadline, remaining time shrinks below the command's natural round-trip
   time, so late polls spuriously timeout via `ExecuteWithTimeout` even
   though the target would have answered "ready" given normal time
   (verified). Root cause: using `time.Until(deadline)` as each attempt's
   bound instead of `min(interval, remaining)` or a fixed per-attempt cap.
   Contrast with `pkg/testtypes/base.go`'s retry+commandTest path, which
   uses a **fixed** per-attempt `t.timeout` independent of the retry
   deadline — that design avoids this specific bug (but has the
   goroutine-leak-per-attempt issue from #2 instead).

4. **Validation asymmetry:** `RetryConfig.Timeout <= 0` is a hard config
   error, but `RetryConfig.Interval <= 0` (including negative) is silently
   replaced with a 2s default — no error, no warning. Also unvalidated:
   `interval > timeout`, which silently degrades retry to a single attempt
   (verified) with no signal to the user that retry never activated.

**Verification method that worked well here:** for concurrency/timing
claims, don't just reason about the code — write a minimal repro in the
same package (reusing existing test helpers like `MockNode`,
`makeStep`/`makeTest`), run with `-race`, capture concrete numbers (call
counts, goroutine deltas, elapsed time), then delete the file. This turned
several "this looks like it could be a problem" hunches into "verified: N
occurrences" findings, and also *disproved* an initial hypothesis (capture
store corruption from stale attempts) that looked plausible from reading
alone but doesn't actually happen because `ExecuteWithTimeout` discards the
abandoned result before it reaches capture-handling code.
