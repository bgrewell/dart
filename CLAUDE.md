# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Rules

- Never mention Claude, Claude Code, AI, or any AI assistant in code, comments, commit messages, or documentation
- Never add co-author lines to commits
- Keep commit messages clean and human-style

## Project Overview

DART (Dynamic Assessment & Regression Toolkit) is a distributed systems testing framework written in Go. It automates the creation of complex, repeatable test scenarios across various environments including local, Docker, LXD/Incus containers, and SSH remotes.

## Build and Development Commands

```bash
# Build binaries (Linux and Windows) - outputs to bin/
make build

# Run directly without building
make run

# Run Go tests
go test ./...

# Run a single test file or package
go test ./internal/config/...
go test -run TestName ./path/to/package

# Download dependencies
make deps

# Clean build artifacts
make clean
```

## Running DART

```bash
# Run with default config.yaml
go run cmd/dart/main.go

# Run with specific config
go run cmd/dart/main.go -c examples/basic/basic.yaml

# Common flags
#   -v, --verbose         Enable verbose output
#   -s, --stop-on-error   Stop on first failure
#   -p, --pause-on-error  Pause for user input on failure
#   -setup, --setup-only  Run only setup steps
#   -teardown, --teardown-only  Run only teardown steps
#   -i, --iterations N    Run test suite N times
```

## Architecture

### Dependency Injection
Uses `go.uber.org/fx` for wiring components. The main entry point (`cmd/dart/main.go`) defines provider functions that fx uses to construct the dependency graph.

### Core Interfaces (`pkg/ifaces/`)
- **Node**: Target environments (local, docker, lxd, ssh) - implements Setup/Teardown/Execute/Close
- **Step**: Setup/teardown operations - implements Run/Title/NodeName
- **Test**: Test definitions - implements Run/Name/NodeName
- **PlatformManager**: Docker/LXD wrapper interface - implements Setup/Teardown/Configured/Name

### Package Structure
- `/cmd/dart/` - Main CLI entry point
- `/pkg/nodetypes/` - Node implementations (local, docker, docker-compose, lxd, ssh, mock)
- `/pkg/steptypes/` - Step implementations (execute, apt, simulated, file_*, http_request, dns_request, service_check)
- `/pkg/testtypes/` - Test implementations (currently only execute)
- `/internal/config/` - YAML configuration loading
- `/internal/controller.go` - TestController orchestrates the entire execution flow
- `/internal/docker/` - Docker platform manager
- `/internal/lxd/` - LXD/Incus platform manager
- `/internal/eval/` - Test result evaluators (exit_code, match, contains)
- `/internal/formatters/` - Console output formatting

### Execution Flow
1. Load YAML configuration
2. Platform setup (Docker networks/images, LXD projects/networks/profiles)
3. Node setup (create containers, SSH connections, etc.)
4. Run setup steps sequentially
5. Run tests, collecting results
6. Run teardown steps sequentially
7. Node teardown
8. Platform teardown (reverse order)
9. Report results and exit with appropriate code

### Test Configuration (YAML)
Tests are defined in YAML files with this structure:
```yaml
suite: Test Suite Name
docker:           # Optional Docker platform config
  networks: [...]
  images: [...]
lxd:              # Optional LXD platform config
  project: {...}
  networks: [...]
  profiles: [...]
nodes:            # Target environments
  - name: nodename
    type: local|docker|docker-compose|lxd|ssh
    options: {...}
setup: [...]      # Pre-test steps
tests:            # Test definitions with evaluators
  - name: test name
    node: nodename
    type: execute
    options:
      command: "..."
      evaluate:
        exit_code: 0
        match: "expected output"
        contains: "substring"
teardown: [...]   # Post-test cleanup steps
```

## Adding New Components

**New Node Type:** Create `pkg/nodetypes/mynode.go` implementing the `Node` interface, add factory logic in `nodetypes.CreateNodesWithWrappers()`

**New Step Type:** Create `pkg/steptypes/mystep.go` implementing the `Step` interface, add factory logic in `steptypes.CreateSteps()`

**New Test Type:** Create `pkg/testtypes/mytest.go` implementing the `Test` interface, add factory logic in `testtypes.CreateTests()`, implement evaluation logic in `internal/eval/`
