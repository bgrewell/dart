---
name: feedback-dart-consistency-review
description: Verified bugs found reviewing feature/consistency (#73, cross-node consistency test type) in DART.
metadata:
  type: project
---

Branch feature/consistency (HEAD "WIP batch F") added the first cross-node
test type. It intentionally breaks the framework-wide "one TestConfig ==
one node after expansion" assumption by skipping `expandTestConfigs` for
`type: consistency` (internal/config/config.go:610). Several older
single-node-assuming code paths were not updated to match, and the new
per-node comparison logic itself has correctness gaps. All findings below
were verified with throwaway repro tests (build/vet/existing suite all
green — these bugs are not caught by the existing test suite).

1. **False-green: `all_equal: false` + an unreachable/erroring node.**
   pkg/testtypes/consistency.go `allEqualCheck.Verify` (~line 210-252)
   groups nodes by output, treating an error as just another distinct
   group key (`<error: ...>`). This makes `equal := len(groups) == 1`
   false whenever any node errors — which trivially satisfies
   `all_equal: false` ("expect nodes to differ") even when every healthy
   node agrees. A doc comment above the type claims "drift and
   unreachability both fail here," which is only true for the default
   `all_equal: true`; it inverts for `all_equal: false`. Repro: 2 healthy
   nodes agreeing + 1 broken MockNode, `evaluate: {all_equal: false}` →
   PASS.

2. **False-green: invalid-UTF-8 stdout can collide after JSON marshal.**
   `gather()` builds `out.Stdout` from raw command bytes, then
   `json.Marshal(report)`. Go's encoding/json replaces invalid UTF-8 with
   U+FFFD when marshaling strings. Two *different* invalid byte sequences
   (e.g. `\xff\xfe` vs `\xfa\xfb`) can both collapse to the identical
   JSON string, making all_equal report "all nodes agree" on genuinely
   different binary output. Repro confirmed with exactly those two byte
   sequences (ASCII-containing invalid sequences don't collide, since the
   valid bytes around the replacement char still differ).

3. **`{{ fact "self" ... }}` resolves to only the first node for a
   consistency test's shared command.** internal/facts/facts.go:202 and
   :217 (`ProcessStepConfigs`/`ProcessTestConfigs`) both do
   `currentNode := cfg.Node[0]` to resolve `self`. This predates
   consistency and assumes cfg.Node has exactly one entry post-expansion.
   Because consistency tests skip expansion, cfg.Node is the full node
   list at fact-render time, so `self` in the one shared command silently
   resolves to node[0]'s facts for every node the command later runs
   against — never updated per-dispatch-target. Confirmed by test:
   `echo {{ fact "self" "ip" }}` with node list [n1,n2] renders using
   n1's IP only.

4. **`worstExit` is "last non-zero seen," not max/worst.**
   pkg/testtypes/consistency.go ~line 171: `if result.ExitCode != 0 {
   worstExit = result.ExitCode }` overwrites rather than taking a max,
   and an Execute() error unconditionally forces `worstExit = 1`
   regardless of a previously-seen larger code. Only matters if a suite
   uses the documented "standard evaluators fall through" feature with
   `evaluate: {exit_code: N}` for a specific non-zero code — default
   `exit_code: 0` still correctly fails. Repro: node exits 5, later node
   (iteration order) exits 2 → synthesized exit code is 2, not 5.

5. **Duplicate node names aren't deduped.** `newConsistencyTest` (line
   ~56-71) never dedupes `nodeNames`/`opts["nodes"]`. A repeated name
   re-executes the command against the same node and double-counts it in
   `matchingCountCheck`, e.g. `count: 1` fails as "2 matching" if the
   duplicated node is the one matching — a config typo masquerades as a
   split-brain/quorum failure instead of a clear validation error.

6. **Reported node (JUnit classname / console) is misleading for
   consistency tests, and can even name a node never queried.**
   pkg/testtypes/base.go:294/313 sets `nodeName := cfg.Node[0]`
   unconditionally in `CreateTests`; consumed by
   internal/report/report.go:178 (`Classname: sanitizeXML(test.Node)`)
   and internal/controller.go (test.NodeName() in console/JSON output).
   For a 3+-node consistency test only the first listed node shows.
   Compounding: `options.nodes` (consistency.go ~line 57-62) fully
   replaces the execution node list without requiring it be a subset of
   `node:`, so `node: [monitor]` + `options.nodes: [n1,n2,n3]` reports
   "monitor" even though monitor was never part of the comparison.

7. **No timeout on the per-node command.** `gather()` calls
   `node.Execute(command)` directly — not `ifaces.BoundedCommand`/
   `ExecuteWithTimeout` (pkg/ifaces/node.go), which every other
   command-driven test type (execute, reboot) uses via a `timeout:`
   option. `newConsistencyTest` never parses a `timeout` key, and the
   framework has no generic unknown-option rejection, so a user-supplied
   `timeout:` is silently accepted and does nothing. A hung command on
   node N blocks every node after it in the (sequential, ordered) loop
   indefinitely.

General method note: the existing consistency_test.go covers the happy
paths (agree/drift/unreachable/leader-election/split-brain/all_equal:false
with genuinely different-but-healthy outputs) well, but has no test
crossing two of those dimensions at once (e.g. all_equal:false +
error, or exit_code evaluator + multiple non-zero codes) — that's where
all four correctness bugs (#1, #2, #4, #5) lived. When reviewing a
"comparison/aggregation across N things" feature, specifically test the
cross product of "one input is an error" x each evaluate mode.
