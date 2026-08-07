# Review: docker platform manager (`internal/docker`)

- **Scope:** `internal/docker/` (wrapper, containers, compose stack +
  registry, image, network) and the docker node teardown path in
  `pkg/nodetypes/docker.go`.
- **Started:** 2026-08-07
- **Status:** complete (2026-08-07)

## Bugs

- [x] **The "environment-dependent" docker test failures were nothing of
  the sort.** `TestListContainers` had been failing (and panicking) since
  before this review series — blamed on needing a Docker daemon. The
  tests are fully mocked; the mock's fixtures set `Status: "running"`
  while the filter (correctly, like the real daemon) matches on `State`.
  One field fix and the entire package passes with no daemon — and the
  full-repo test run needs no exclusions anymore.

- [x] **`--teardown-only` could not address any docker resource.** Every
  wrapper operation resolved names through in-process `name → ID` maps
  populated only during Setup; a teardown-only run starts with them
  empty, so stop/remove/network calls targeted `""`. Lookups now fall
  back to the name itself (the Docker API accepts names wherever it
  accepts IDs).

- [x] **Teardown failed hard on already-missing resources** — same class
  as the LXD fix: an exported `IsNotFound` (typed containerd errdefs +
  string forms) makes wrapper teardown and the docker node teardown
  treat missing containers/networks/images as already removed.

## Notes

- `BuildImage` shells out to the `docker build` CLI while everything else
  uses the SDK — works (env-driven config matches), but requires the CLI
  installed and skips the negotiated API client. Migrating to the SDK's
  ImageBuild (with build-context tarring) is a self-contained future
  improvement.
- `ConnectContainerToNetwork` / `DisconnectContainerFromNetwork` /
  `ModifyContainerGateway` / `RemoveNetworkIsolationRules` are
  not-implemented stubs; nothing calls them. Candidates for deletion in a
  later pass.
- The compose stack registry (refcounted sharing of stacks across nodes)
  read clean and is already tested.

## Resolution log

- **2026-08-07** — Findings resolved on branch `review/docker`. The
  package's tests now run (and pass) everywhere; `IsNotFound` has its own
  test. No local daemon was available to smoke the container lifecycle —
  covered by the mocked suites; flagged for a daemon-backed run when one
  is handy.
