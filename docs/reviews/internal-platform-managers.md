# Review: platform managers (`internal/lxd`, `internal/lxc`, `internal/platform`)

- **Scope:** the LXD/Incus API wrapper (`internal/lxd`), the legacy lxc
  CLI helper (`internal/lxc`), runtime detection (`internal/platform`).
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **Platform teardown failed hard on already-missing resources.**
  Removing a network or profile that no longer exists (partial setup, a
  previous run's cleanup, manual deletion) errored and aborted the rest
  of teardown — the same failure class fixed for instances in #47.
  *Fix:* a shared `IsNotFound` helper (typed 404 + string form); teardown
  treats missing profiles/networks as already removed. The duplicated
  inline check in the LXD node teardown now uses the same helper.
  Verified against the real daemon: a second `--teardown-only` run with
  the resources already gone completes cleanly.

- [x] **Teardown removed networks before profiles.** Profiles can
  reference networks, so deletion could fail on the dependency. Teardown
  now runs in reverse creation order (profiles, then networks).

- [x] **Malformed bridge subnet/gateway reached the server.** A bad
  `subnet:` fell back to `/24` silently and an invalid gateway produced
  an obscure server-side error. `CreateBridgeNetwork` validates CIDR and
  IP up front with clear messages.

## Notes

- `internal/lxc` (legacy CLI helper) and `internal/platform` (runtime
  detection with cached results) were read end to end — no findings; both
  already have test coverage (100% / relevant paths).
- `Wrapper.ExecuteInInstance` hardcodes `/bin/bash` but has no callers in
  the repo — candidate for deletion in a later pass; left because the
  wrapper is a `pkg`-consumed API surface.
- The wrapper's unused `networkNamesToId`/`instanceNamesToId` maps mirror
  names to themselves; harmless bookkeeping, left as-is.
- Real coverage of the instance lifecycle lives in the end-to-end LXD
  runs (reboot suite, platform smoke) — unit-mocking the
  `lxd.InstanceServer` interface is not worth its size.

## Resolution log

- **2026-08-07** — Findings resolved on branch `review/platform-managers`.
  Unit tests cover bridge validation (pre-server) and `IsNotFound`
  (typed and string forms). Real-daemon smoke: network create → test →
  teardown → repeat teardown with resources missing → clean exit.
