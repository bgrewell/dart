# DART Documentation

Reference documentation for DART. Start with the
[project README](../README.md) if you are new — it covers installation
and the common workflows; these pages are the complete details.

| Page | Contents |
|---|---|
| [Node types](node-types.md) | `local`, `docker`, `docker-compose`, `lxd`, `ssh`: every option, remote daemons, ISO boot, security defaults |
| [Test types](tests.md) | Every test type, plus retries, timeouts, skips, captures, variables, and tags |
| [Evaluation reference](evaluation.md) | Every `evaluate` check: exit codes, output matching, numeric bounds, JSON paths, timing |
| [Setup and teardown steps](steps.md) | Every step type: commands, packages, files, templates, snapshots, waits |
| [Command line](cli.md) | Flags, exit codes, reports, and CI integration |

## Working notes

- [Code reviews](reviews/) — findings and resolutions from the review
  passes over each area of the codebase, kept so decisions aren't
  re-litigated.
