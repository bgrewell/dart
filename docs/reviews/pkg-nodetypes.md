# Review: node types (`pkg/nodetypes`)

- **Scope:** `pkg/nodetypes/` — ssh, docker, docker-compose, local, lxd
  node implementations and the node factory. (`local.go` was largely
  covered by the #38 fix; `MockNode` by the steptypes review; the LXD
  node's platform logic belongs to the `internal/lxd` review.)
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **SSH command output was entirely lost.**
  `Execute` returned `session.StdoutPipe()`/`StderrPipe()` readers, but
  `session.Run` drains and closes those pipes before returning — every
  consumer saw empty output, so any `match`/`contains`/fact/capture
  against an SSH node could never see real data. (The defect was
  documented in a TODO, fix included.) *Fix:* output is captured via tee
  writers assigned as the session's Stdout/Stderr — the same pattern the
  LXD node uses — which also gives SSH debug streaming. Verified by a new
  integration test against an in-process SSH server, which the old code
  fails.

- [x] **SSH sessions were never closed** — one leaked per command on
  long-lived connections. `defer session.Close()` added.

- [x] **`DockerNode.Close()` returned "not implemented" as an error.**
  Harmless while the controller dropped close errors; fatal-looking now
  that `Close` errors are aggregated and reported. There is nothing to
  release (container lifecycle is Setup/Teardown, the client belongs to
  the shared wrapper) — returns nil.

- [x] **`lxd-vm` alias mutated the node's config options map** (wrote
  `instance_type` into `cfg.Options`). Copies the map instead.

- [x] **SSH nodes had no name** — the `address` field was never set
  before the reboot work, and debug output had no node identity. Nodes
  now carry their config name (factory passes it through) for tee-writer
  prefixes.

## Notes, kept as-is

- SSH uses `InsecureIgnoreHostKey` — host keys are not verified. Fine for
  lab/test networks that DART targets; a `host_key`/`known_hosts` option
  is a candidate feature if DART ever points at hosts worth
  authenticating.
- SSH `exec_opts` (shell/env) are ignored: commands run through the
  remote user's shell. Documented behavior.
- Docker teardown errors when the container never got created (partial
  setup); tolerance for missing containers belongs to the
  `internal/docker` review.
- The factory's duplicate-node check is now redundant with config
  validation but harmless (belt and braces for programmatic callers).

## Resolution log

- **2026-08-07** — All findings resolved on branch `review/nodetypes`.
  New in-process SSH server test suite exercises stdout/stderr capture,
  exit codes, sequential sessions, and auth failure — the capture tests
  fail against the previous pipe-based implementation.
