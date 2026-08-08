# DART - Dynamic Assessment & Regression Toolkit

> **Notice:** This project is in an early development phase and may not yet be fully stable or feature complete. As it evolves, you may encounter significant changes to the API, behavior, and overall functionality.

DART is a testing framework built to simplify the creation of complex, repeatable test scenarios across a variety of environments. Whether you're validating a single service or coordinating distributed systems, DART empowers you to automate environment setup, execution, and cleanup with minimal effort. Moreover, it integrates effortlessly into existing projects, enabling developers to include test definitions directly within their repositories so that upon cloning, they can immediately verify that all components are functioning as intended.

---

## Table of Contents

1. [Overview](#overview)  
2. [Key Features](#key-features)  
3. [Node Types](#node-types)  
4. [Setup and Teardown Tasks](#setup-and-teardown-tasks)  
5. [Installation](#installation)  
6. [Usage](#usage)  
   - [Command Line Reference](#command-line-reference)  
   - [Exit Codes](#exit-codes)  
7. [Example Test Execution](#example-test-execution)  
8. [Example Test Definition](#example-test-definition)  
9. [Test Types](#test-types)  
10. [Test Evaluation Reference](#test-evaluation-reference)  
10. [License](#license)  

---
 
## Overview

DART addresses the challenges of distributed systems testing by structuring workflows into **nodes**, **setup steps**, **tests**, and **teardown steps**. It supports various node types—from local processes and SSH remotes to Docker/LXD containers and virtual machines—while automating the configuration and testing processes. Its declarative YAML configuration allows you to embed test definitions directly within your project, so when you clone a repository, you can instantly run the tests to verify that your local environment is configured correctly.

---

## Key Features

- **Multiple Node Types**  
  Operates with localhost, remote SSH systems, containers (Docker/LXD), and virtual machines.

- **Automated Environment Preparation**  
  Provisions and configures nodes automatically, enabling on-demand creation of containers and virtual machines.

- **Declarative YAML Configuration**  
  Define your test suites in clear, maintainable YAML files that cover node configuration, setup, execution, and teardown.

- **Seamless Integration**  
  Easily embed test definitions within your existing projects so that a simple clone can yield a fully testable environment.

- **Setup and Teardown Hooks**  
  Run pre- and post-test operations to maintain a predictable and stable testing state.

- **Human-Readable Output**  
  Provides clear, color-coded test feedback, making it easy to see results at a glance.

- **DevOps Friendly**  
  Returns an exit code that reflects the outcome of the tests, integrating smoothly with CI/CD pipelines.

---

## Node Types

DART supports several types of nodes that can be used as test targets:

- **Local Node (`local`)**  
  Execute tests on the local machine where DART is running.

- **Docker Node (`docker`)**  
  Run tests inside Docker containers, with support for custom networks and privileged mode. Supports both local and remote Docker hosts.

- **Docker Compose Node (`docker-compose`)**  
  Manage and test services defined in Docker Compose files. Multiple nodes can target different services in the same compose stack.

- **LXD Node (`lxd`)**  
  Execute tests in LXD containers or virtual machines, with automatic provisioning and cleanup. Supports both local Unix socket connections and remote HTTPS connections with certificate-based authentication.

- **SSH Node (`ssh`)**  
  Run tests on remote machines via SSH, supporting both password and key-based authentication.

Each node type can be configured with specific options in your YAML configuration file. For example:

```yaml
nodes:
  - name: localhost
    type: local
    options:
      shell: /bin/bash
  
  - name: remote-server
    type: ssh
    options:
      host: example.com
      port: 22
      user: testuser
      key: ~/.ssh/id_rsa

  - name: test-container
    type: docker
    options:
      image: ubuntu:latest
      networks:
        - name: test-net
          subnet: "172.20.0.0/16"
          ip: "172.20.0.2"
  
  # Docker Compose nodes - target specific services
  - name: web-service
    type: docker-compose
    options:
      compose_file: docker-compose.yml
      project_name: my-stack
      service: web
  
  - name: db-service
    type: docker-compose
    options:
      compose_file: docker-compose.yml
      project_name: my-stack
      service: db
  
  # Remote LXD node with certificate authentication
  - name: remote-lxd
    type: lxd
    options:
      remote_addr: https://10.0.0.1:8443
      client_cert: ~/.config/lxc/client.crt
      client_key: ~/.config/lxc/client.key
      image: ubuntu:24.04
      instance_type: container
```

### Remote Docker Support

Docker nodes can connect to remote Docker hosts using standard Docker environment variables:

- **DOCKER_HOST**: URL to the Docker server (e.g., `tcp://remote-host:2376` or `ssh://user@host`)
- **DOCKER_TLS_VERIFY**: Enable TLS verification (set to `1`)
- **DOCKER_CERT_PATH**: Path to directory containing TLS certificates (`ca.pem`, `cert.pem`, `key.pem`)

Example:
```bash
export DOCKER_HOST=tcp://10.0.0.1:2376
export DOCKER_TLS_VERIFY=1
export DOCKER_CERT_PATH=/path/to/certs
dart -c config.yaml
```

For SSH-based connections (no Docker daemon configuration needed):
```bash
export DOCKER_HOST=ssh://user@remote-host
dart -c config.yaml
```

See `examples/docker/docker-remote.yaml` for a complete example.

### Remote LXD Support

LXD nodes support remote connections using modern trust token authentication or traditional certificate-based authentication.

**Trust Token Authentication (Recommended):**

Configure the remote LXD server and generate a trust token:

```bash
# On the remote LXD server
lxc config set core.https_address "[::]:8443"
lxc config trust add dart-client
# Copy the generated token
```

Use the token in your DART configuration:

```yaml
nodes:
  - name: remote-container
    type: lxd
    options:
      remote_addr: https://10.0.0.1:8443
      trust_token: eyJjbGllbnRfbmFtZSI6ImRhcnQtY2xpZW50...
      image: ubuntu:24.04
      instance_type: container
```

**Certificate-Based Authentication (Traditional):**

Generate client certificates:
```bash
# Add remote server and generate certificates
lxc remote add myremote https://remote-server-ip:8443
```

Use the generated certificates in your DART configuration:

```yaml
nodes:
  - name: remote-container
    type: lxd
    options:
      remote_addr: https://10.0.0.1:8443
      client_cert: ~/.config/lxc/client.crt
      client_key: ~/.config/lxc/client.key
      # Optional: server_cert for custom CA
      # Optional: skip_verify: true (not recommended for production)
      image: ubuntu:24.04
      instance_type: container
```

See `examples/lxd/lxd-remote.yaml` for a complete example.

### Empty VMs and ISO Boot

LXD nodes are normally created from an image. To test an installer instead, create an empty
virtual machine and attach the ISO as a boot device. DART creates the instance with no source,
attaches the devices, starts it, and then waits for the install to finish rather than expecting
the instance to answer right away.

```yaml
nodes:
  - name: iso-vm
    type: lxd
    options:
      instance_type: virtual-machine

      # Create the VM with no image; it boots from its devices instead
      empty: true

      # Instance configuration keys, applied at creation
      config:
        security.secureboot: "false"

      # Devices attached to the instance
      devices:
        iso:
          type: disk
          source: work/output/example-0.1.0-amd64.iso
          # Rank the ISO above the root disk so the installer boots first
          boot.priority: 10

      # An installing VM is unreachable until it reboots from disk, so poll for it
      boot_wait:
        timeout: 1800          # Maximum seconds to wait (default 300)
        interval: 15           # Seconds between checks (default 2)
        initial_delay: 60      # Seconds before the first check (default 0)
        ready_command: cat /etc/hostname
```

Notes:

- `empty: true` creates the instance with no source. Omitting `image` has the same effect;
  setting both `empty: true` and `image` is rejected.
- `devices` accepts any LXD device configuration and is merged over the NICs generated from
  `networks`, so a node can override a generated device if it needs to.
- Relative `source` paths on disk devices are resolved to absolute paths, letting a test file
  reference build artifacts by their path in the repository. Sources on remote nodes are paths
  on the remote server and are passed through untouched.
- `boot_wait` replaces the default readiness check: DART polls `ready_command` through the
  configured shell until it exits zero or the timeout expires. Without `ready_command`, being
  able to run any command at all counts as ready.
- An empty instance with no `boot_wait` is started and left alone, since it has no guest agent
  to wait for.

See `examples/lxd/lxd-iso-vm.yaml` for a complete example.

### LXD/Incus Auto-Detection

DART automatically detects whether the host system has LXD or Incus installed and configures the appropriate socket path. This allows test configurations to be portable across systems without modification.

**Detection Priority:**
1. `/var/lib/incus/unix.socket` (Incus)
2. `/var/snap/lxd/common/lxd/unix.socket` (LXD snap)
3. `/var/lib/lxd/unix.socket` (LXD native)

**Image Name Translation:**

When Incus is detected, DART automatically translates LXD-style image references:
- `ubuntu:24.04` becomes `images:ubuntu/24.04`
- `images:debian/12` remains unchanged

**Limitations:**

This auto-detection provides basic compatibility but has limitations. For production use or complex scenarios, we recommend configuring your test definitions explicitly for your target virtualization platform:

```yaml
# Explicit socket configuration (recommended for production)
lxd:
  socket: /var/lib/incus/unix.socket

nodes:
  - name: test-container
    type: lxd
    options:
      image: images:ubuntu/24.04  # Use Incus-native format
      instance_type: container
```

### LXD Project Support

LXD projects provide resource isolation and organization within LXD. DART supports creating and managing LXD projects, automatically copying the default profile, and organizing instances, networks, and profiles within projects.

**Benefits of Using Projects:**
- **Resource Isolation**: Separate test environments without conflicts
- **Organization**: Group related resources together
- **Easy Cleanup**: Delete all resources by removing the project
- **Multi-tenancy**: Run multiple test suites in parallel

**Configuring a Project:**

```yaml
lxd:
  project:
    name: dart-test-project
    description: Test project for integration tests
    config:
      features.images: "true"
      features.profiles: "true"
      features.networks: "true"
      features.storage.volumes: "true"
  
  # Networks are created within the project
  networks:
    - name: test-network
      type: bridge
      subnet: 10.100.0.0/24
      gateway: 10.100.0.1
  
  # Profiles are created within the project
  # The default profile is automatically copied
  profiles:
    - name: custom-profile
      description: Custom profile for tests
      config:
        limits.cpu: "2"
        limits.memory: "2GB"

nodes:
  - name: test-container
    type: lxd
    options:
      # Will use the project defined in lxd.project
      image: ubuntu:24.04
      instance_type: container
      # Or explicitly specify a project
      # project: dart-test-project
```

**Important Notes:**
- The default profile is automatically copied to new projects
- All resources (instances, networks, profiles) are automatically cleaned up when the project is deleted
- Projects are created during setup and deleted during teardown

See `examples/lxd/lxd-project.yaml` for a complete example.


## Setup and Teardown Tasks

Setup and teardown tasks in DART are specialized operations designed to prepare and clean up test environments. Unlike tests, which validate functionality and return pass/fail results, these tasks focus on environment management and are considered successful if they complete without errors.

### Purpose and Execution Flow

1. **Setup Tasks**
   - Run before any tests begin
   - Prepare the test environment (e.g., installing dependencies, configuring services)
   - Must complete successfully for tests to begin
   - Run in sequence to ensure proper initialization

2. **Teardown Tasks**
   - Run after all tests complete (or after a critical failure)
   - Clean up resources and restore system state
   - Execute even if tests fail (ensuring proper cleanup)
   - Run in sequence to ensure proper cleanup order

### Key Differences from Tests

- **Success Criteria**: Tasks succeed/fail based on completion, while tests evaluate specific conditions
- **Evaluation**: Tasks don't have evaluation criteria like `match` or `contains`
- **Error Handling**: Task failures stop the entire suite, while test failures can be configured to continue
- **Scope**: Tasks affect the environment, while tests validate functionality
- **Timing**: Tasks run before/after all tests, while tests run in the middle phase

### Available Task Types

#### Execute Task (`execute`)
Run shell commands on the target node. Ideal for custom setup operations.

```yaml
- name: configure database
  node: db-server
  step:
    type: execute
    options:
      command: |
        mysql -u root -e "CREATE DATABASE testdb;"
        mysql -u root -e "GRANT ALL ON testdb.* TO 'testuser'@'%';"
```

#### APT Package Management (`apt`)
Manage system packages on Debian-based systems. Handles updates and dependencies automatically.

```yaml
- name: install system dependencies
  node: test-container
  step:
    type: apt
    options:
      packages:
        - nginx
        - postgresql
        - redis-server
```

#### Simulated Task (`simulated`)
Add controlled delays in the setup/teardown process. Useful for:
- Waiting for services to initialize
- Simulating network delays
- Testing timing-dependent scenarios

```yaml
- name: wait for service initialization
  node: app-server
  step:
    type: simulated
    options:
      time: 5  # Seconds; fractional values like 0.5 are allowed
```

#### File Operations (`file_create`, `file_edit`, `file_delete`, `file_exists`, `file_read`)
Create, modify, verify, and remove files **on the step's target node** —
local nodes use the native filesystem, container/SSH nodes are driven
through their shell (requires standard POSIX tools on the node).
`file_write` is an alias for `file_create`.

```yaml
- name: write app config
  node: test-container
  step:
    type: file_create
    options:
      path: /etc/myapp/config.ini
      contents: |
        [server]
        port = 8080
      create_dir: true       # mkdir -p the parent directory
      overwrite: true        # without this, an existing file is an error
      mode: "0640"           # octal; quote it or use a leading zero

- name: point app at test database
  node: test-container
  step:
    type: file_edit
    options:
      path: /etc/myapp/config.ini
      operation: replace     # insert | replace | remove
      match_type: plain      # plain | regex | line
      match: "port = 8080"
      content: "port = 9090"
      # insert also takes position: before|after; line takes line_number;
      # regex replace supports use_captures with $1 / ${name} references

- name: verify config content
  node: test-container
  step:
    type: file_read
    options:
      path: /etc/myapp/config.ini
      contains: "port = 9090"

- name: cleanup
  node: test-container
  step:
    type: file_delete
    options:
      path: /etc/myapp/config.ini
      ignore_errors: true    # missing file is not a failure
```

#### File Transfer and Templating (`file_push`, `file_fetch`, `file_template`)
Deploy repository files onto a node, pull artifacts back, or render one
template per node. Local nodes use the filesystem directly; container and
SSH nodes are driven through their shell.

```yaml
setup:
  - name: deploy the service binary
    node: app-server
    step:
      type: file_push
      options:
        source: build/myservice          # path on the machine running DART
        dest: /usr/local/bin/myservice
        overwrite: true
        create_dir: true                 # mode defaults to the source's

  - name: render per-node config
    node: app-server
    step:
      type: file_template
      options:
        source: fixtures/app.conf.tmpl   # Go template: {{ .port }}
        dest: /etc/myapp/app.conf
        overwrite: true
        mode: "0640"
        values:
          port: 8080
          backend: "{{ fact \"db\" \"ipv4\" }}"

teardown:
  - name: keep the logs for triage
    node: app-server
    step:
      type: file_fetch
      options:
        source: /var/log/myapp.log
        dest: artifacts/myapp.log
        create_dir: true
```

Templates are parsed at config load, so a broken one fails before the run
starts, and a value that is missing or null is an error rather than a
silently empty (or literal `<no value>`) config line. `file_fetch` refuses
to overwrite an existing local file unless `overwrite: true`, so a fetched
artifact cannot clobber a previous run's. Content to container and SSH
nodes is written in chunks, so files are not limited by the shell's
per-argument size cap.

#### Snapshots (`snapshot`)
Give destructive tests cheap isolation on LXD nodes: capture state in
setup, break things, roll back in teardown — far faster than recreating
a node.

```yaml
setup:
  - name: capture clean state
    node: iso-vm
    step:
      type: snapshot
      options:
        name: clean          # action defaults to create
        # stateful: true     # include running memory (needs CRIU)

Restoring a running instance stops and restarts it; DART blocks until the
node accepts commands again, so a following step cannot race the reboot.

teardown:
  - name: roll back
    node: iso-vm
    step:
      type: snapshot
      options: { name: clean, action: restore }
      # A snapshot taken with stateful: true must also be restored with
      # stateful: true — otherwise LXD performs a disk-only restore and
      # silently discards the saved memory.
  - name: clean up the snapshot
    node: iso-vm
    step:
      type: snapshot
      options: { name: clean, action: delete }
```

#### Service Check (`service_check`)
Verify a systemd service is active on the target node.

```yaml
- name: ensure nginx is running
  node: web-server
  step:
    type: service_check
    options:
      service: nginx
```

#### HTTP Request (`http_request`)
Perform an HTTP request and validate the response. The request is made from
the host running DART (verifying reachability from the controller), not from
the node.

```yaml
- name: check API health endpoint
  node: local
  step:
    type: http_request
    options:
      url: http://localhost:8080/health
      method: GET              # default GET
      expected_status: 200     # default 200
      expected_body: healthy   # optional substring check
      timeout: 5               # seconds, default 30
```

#### DNS Request (`dns_request`)
Resolve a hostname (using the DART host's resolver) and optionally verify
expected addresses appear in the answers.

```yaml
- name: verify service DNS
  node: local
  step:
    type: dns_request
    options:
      hostname: db.test.internal
      expected_ips:
        - 10.0.0.5
```

### Best Practices

1. **Environment Isolation**
   - Use setup tasks to create isolated test environments
   - Ensure teardown tasks clean up ALL created resources
   - Avoid leaving behind test artifacts

2. **Idempotency**
   - Design tasks to be repeatable
   - Handle cases where resources may already exist
   - Ensure clean state regardless of previous runs

3. **Error Handling**
   - Include error checking in setup tasks
   - Implement proper cleanup in teardown tasks
   - Log relevant information for debugging

4. **Resource Management**
   ```yaml
   setup:
     - name: create test directory
       node: test-server
       step:
         type: execute
         options:
           command: "mkdir -p /tmp/test-data"
   
   teardown:
     - name: cleanup test directory
       node: test-server
       step:
         type: execute
         options:
           command: "rm -rf /tmp/test-data"
   ```

### Planned Future Task Types

DART is actively developing additional task types to enhance environment management:

- **SNAP Package Management**
  - Install/remove snap packages
  - Configure snap services

- **Git Operations**
  - Clone repositories
  - Checkout specific branches/tags
  - Apply patches

- **Network Configuration**
  - Configure network interfaces
  - Set up routing rules
  - Manage firewall settings

- **Service Management**
  - Start/stop system services
  - Configure service parameters
  - Manage service dependencies

---

## Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/bgrewell/dart/main/install.sh | bash
```

This installs the latest release to `/usr/local/bin/dart`. You can customize the install with environment variables:

```bash
# Custom install directory
DART_INSTALL_DIR=~/.local/bin curl -sSL https://raw.githubusercontent.com/bgrewell/dart/main/install.sh | bash

# Specific version
DART_VERSION=v0.4.0 curl -sSL https://raw.githubusercontent.com/bgrewell/dart/main/install.sh | bash
```

### Go Install

If you have Go installed, you can install directly:

```bash
go install github.com/bgrewell/dart/cmd/dart@latest
```

### Manual Download

Download a binary from the [releases page](https://github.com/bgrewell/dart/releases) and place it on your `PATH`:

```bash
chmod +x dart-linux-amd64
sudo mv dart-linux-amd64 /usr/local/bin/dart
```

Each release includes a Software Bill of Materials (SBOM) and Vulnerability Scan (VEX) results.

### Build from Source

Requires Go 1.23+:

```bash
git clone https://github.com/bgrewell/dart.git
cd dart
make build
# Binary is at bin/dart
```

---

## Usage

### Command Line Reference

```bash
Usage: dart [OPTIONS] [ARGUMENTS]

Version: dev
Date: dev
Codebase: dev (dev)

Description: DART is a distributed systems testing framework
  designed to make it easy to perform automation and
  integration testing on a wide variety of distributed
  systems.

Options:
  Default: Default Options
    -c        --config          config.yaml  The path to the configuration file
    -v        --verbose         false        Enable verbose output
    -p        --pause-on-error  false        Pause on error
    -s        --stop-on-error   false        Stop on error
    -setup    --setup-only      false        Only run the setup steps
    -teardown --teardown-only   false        Only run the teardown steps
```

### CI Integration

```bash
dart -c suite.yaml -r junit:results.xml,json:results.json   # test panels + tooling
dart -c suite.yaml -l run.log                               # clean transcript (no colors/spinners)
dart -c suite.yaml --check                                  # validate config + print plan, run nothing
```

JUnit output feeds GitHub/GitLab/Jenkins test panels (skips and failure
details included); JSON carries the same data plus durations for custom
tooling. `--check` validates node types, report specs, and the full option set of
every step and test against mock nodes — a pre-commit or CI lint that
touches no infrastructure (node connectivity is not exercised). The results summary shows total suite time. With `-i N`, each iteration
writes its own report (`results-1.xml`, `results-2.xml`, ...) so a passing
final iteration can't mask an earlier failure; reports are also written
when a run aborts early (teardown failure, stop-on-error), and `--log`
captures debug-streamed command output too.

### Exit Codes

- **0**: All tests passed successfully.
- **Non-zero**: One or more tests failed or an unexpected error occurred.

These exit codes allow DART to integrate with automated DevOps workflows, ensuring that issues are immediately flagged during continuous integration and deployment processes.

---

## Example Test Execution

Below is a simplified example of how DART logs its operations during a test run. The actual output includes color coding and more detailed formatting for clarity:

```bash
[+] Running test setup
  running setup ...................... done 
  running setup ...................... done 
  ensure sshpass is installed ........ done 
  ensure dns is working .............. done 
  install locker ..................... done 
  create user bob .................... done 
  create user jim .................... done 
  create user tom .................... done 
  ensure password login is allowed ... done 
  restart ssh ........................ done 

[+] Running tests
  00001: verify locker is installed .................. passed
  00002: ssh to locker-test as bob ................... passed
  00003: ssh to locker-test as jim ................... passed
  00004: lock system as jim .......................... passed
  00005: ssh to locker-test as disallowed user bob ... passed
  00006: ssh to locker-test as allowed user tom ...... passed
  00007: unlock system as jim ........................ passed
  00008: verify bob can again access the system ...... passed

[+] Running test teardown
  running teardown ................... done 
  running teardown ................... done 

[+] Results
  Pass: 00008
  Fail: 00000
```

---

## Example Test Definition

The YAML configuration below demonstrates how to define nodes, setup steps, tests, and teardown operations. This example provisions and tests a tool called `locker` in an LXD container:

```yaml
---
suite: Locker End-to-End Tests
nodes:
  - name: localhost
    type: local
    options:
      shell: /bin/bash
  - name: locker-test
    type: lxd
    options:
      image: ubuntu:24.04
      type: container

setup:
  - name: ensure sshpass is installed
    node: localhost
    step:
      type: apt
      options:
        packages:
          - sshpass

  - name: ensure dns is working
    node: locker-test
    step:
      type: execute
      options:
        command: 'until nslookup github.com &>/dev/null; do sleep 1; done'

  - name: install locker
    node: locker-test
    step:
      type: execute
      options:
        command: "bash -o pipefail -c 'curl -fSL https://bgrewell.github.io/locker/install.sh | bash'"

  - name: create user bob
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash bob && echo 'bob:password123' | chpasswd"

  - name: create user jim
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash jim && echo 'jim:password123' | chpasswd"

  - name: create user tom
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash tom && echo 'tom:password123' | chpasswd"

  - name: ensure password login is allowed
    node: locker-test
    step:
      type: execute
      options:
        command: "rm /etc/ssh/sshd_config.d/60-cloudimg-settings.conf"

  - name: restart ssh
    node: locker-test
    step:
      type: execute
      options:
        command: "systemctl restart ssh"

tests:
  - name: verify locker is installed
    node: locker-test
    type: execute
    options:
      command: "locker -h"
      evaluate:
        exit_code: 0

  - name: test
    node: localhost
    type: execute
    options:
      command: "whoami"
      evaluate:
        match: "ben"

  - name: ssh to locker-test as bob
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') whoami"
      evaluate:
        match: "bob"
        exit_code: 0

  - name: ssh to locker-test as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') whoami"
      evaluate:
        match: "jim"
        exit_code: 0

  - name: lock system as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') locker -r test -u tom lock"
      evaluate:
        contains: "Lock acquired"
        exit_code: 0

  - name: ssh to locker-test as disallowed user bob
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        exit_code: 255

  - name: ssh to locker-test as allowed user tom
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no tom@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        match: test
        exit_code: 0

  - name: unlock system as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') unlock"
      evaluate:
        contains: "Lock released"
        exit_code: 0

  - name: verify bob can again access the system
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        match: test
        exit_code: 0
```

---

## Test Types

Tests run against a node and evaluate the outcome. `execute` is the
general-purpose type; the others are shorthand for common checks and accept
the same `evaluate` keys where noted.

| Type | What it does | Key options |
|------|--------------|-------------|
| `execute` | Run a command on the node, evaluate its result | `command`, `evaluate` (full reference below) |
| `exists` | Check a path exists on the node (`test -e`) | `path`; `evaluate.exists: true\|false` |
| `file_content` | Read a file on the node, evaluate its content | `filename`; standard `evaluate` keys apply to the content |
| `file_hash` | Verify file checksums on the node | `filename`; `evaluate.md5/sha1/sha256` hex digests |
| `service_status` | Check a systemd unit state on the node | `service`; `evaluate.status` (default `active`) |
| `ping` | Ping a target from the node | `target`, `count`; `evaluate.packet_loss` (max %), `rtt_min/rtt_avg/rtt_max` (ms) |
| `http_request` | HTTP request from the DART host | `url`, `method`, `headers`, `timeout`; `evaluate.status_code` plus standard keys against the body |
| `port_check` | TCP connect from the DART host | `host`, `port`, `timeout`; `evaluate.status: open\|closed` |
| `reboot` | Restart the node mid-suite and wait until it accepts commands | `mode: graceful\|force`, `ready_command`, `timeout` (lxd and ssh nodes) |
| `consistency` | Compare one command's output **across** nodes | `command`, `nodes`; `evaluate.all_equal`, `matching: {pattern, count}` |
| `tls_cert` | Inspect a TLS endpoint's certificate | `host`, `port` (443), `server_name`, `timeout`; `evaluate.min_days_remaining`, `dns_names`, `issuer_contains`, `subject_contains`, `chain_valid` |

Any test also accepts test-level `retry:` (see Timeouts and Retries) and
`skip_if`/`skip_unless` (see Conditional Skips).

```yaml
tests:
  - name: reboot to apply rollback
    node: iso-vm
    type: reboot
    options:
      mode: force            # model a power cut; graceful is the default
      ready_command: cat /etc/hostname
      timeout: 600
```

`reboot` is also available as a setup/teardown step with the same
options. On LXD nodes the readiness wait reuses the node's `boot_wait`
configuration; `mode: force` kills the instance without a clean shutdown,
which is what crash-safety suites need. On SSH nodes the reboot is issued
over the session (passwordless sudo or root) and DART reconnects until the
host answers again.

```yaml
tests:
  - name: API serves healthy status
    node: app-server
    type: http_request
    options:
      url: http://localhost:8080/health
      evaluate:
        status_code: 200
        json_path:
          path: status
          equals: healthy

  - name: config deployed with right hash
    node: app-server
    type: file_hash
    options:
      filename: /etc/myapp/config.ini
      evaluate:
        sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

  - name: database reachable with low latency
    node: app-server
    type: ping
    options:
      target: db.internal
      count: 5
      evaluate:
        packet_loss: 0
        rtt_max: 10
```

Note: `ping`, `exists`, `file_content`, `file_hash`, and `service_status`
run commands on the target node (POSIX tools assumed); `http_request`,
`tls_cert`, and `port_check` act from the host running DART unless told
otherwise.

### Network Reachability and Certificates

`port_check` answers firewall and ACL questions when pointed at a node:
`from: node` runs the probe on the node's own shell, so a suite can assert
both that permitted paths work and that blocked ones stay blocked. The
probe prefers bash's `/dev/tcp` and falls back to `nc` only after proving
the node's build accepts `-z` (busybox builds often don't); a node with no
usable method reports `unsupported` and fails the check rather than
guessing. Host values are passed as arguments, never interpolated into a
shell string.

```yaml
tests:
  - name: app server reaches the database
    node: app
    type: port_check
    options:
      host: db.internal
      port: 5432
      from: node          # probe from the node, not the DART host

  - name: app server cannot reach admin SSH
    node: app
    type: port_check
    options:
      host: admin.internal
      port: 22
      from: node
      evaluate:
        status: closed    # negative policy assertions

  - name: gateway certificate is not about to expire
    node: local
    type: tls_cert
    options:
      host: vault.internal
      evaluate:
        min_days_remaining: 30
        chain_valid: true
        dns_names: [vault.internal]
```

Certificate facts are emitted as JSON, so `json_path`, `contains`, and the
other standard evaluators work against them too. Inspection deliberately
skips chain verification during the handshake, so expired or misissued
certificates are still inspectable — assert `chain_valid` explicitly.

### Cluster Consistency

Config drift and quorum questions compare nodes *with each other*, which
per-node tests cannot express. A `consistency` test runs one command
everywhere and compares the results; unlike other types its `node:` list
is not expanded into separate tests.

```yaml
tests:
  - name: every node runs the same config
    node: [web-1, web-2, web-3]
    type: consistency
    options:
      command: sha256sum /etc/app.conf
      # all_equal: true is the default check

  - name: exactly one leader is elected
    node: [db-1, db-2, db-3]
    type: consistency
    options:
      command: cluster-role
      evaluate:
        matching:
          pattern: "^leader$"
          count: 1              # 2 leaders (split brain) or 0 both fail

  - name: nodes keep distinct identities
    node: [web-1, web-2]
    type: consistency
    options:
      command: hostname
      evaluate:
        all_equal: false
```

Failures name which nodes disagree and what each returned
(`web-1,web-2 => "v3" | web-3 => "v2"`). A node that cannot run the
command fails the comparison in **both** directions — an outage never
satisfies `all_equal: false` — and comparison is by content digest, so
binary outputs cannot collapse into false agreement. `timeout:` bounds
each node's command, and the per-node report is emitted as JSON so
`json_path` and the standard evaluators apply too.

Note: `{{ fact "self" ... }}` is rejected in a consistency command —
one command runs on many nodes, so "self" has no single meaning; name
the node explicitly.

### Built-in Network Facts

LXD and Docker nodes report their own addresses without a fact command:
`{{ fact "web" "ipv4" }}`, `{{ fact "web" "ipv6" }}`, and per-interface or
per-network variants (`ipv4.eth0`, `ipv4.test-net`). User-defined facts of
the same name win, and discovery failures never fail a run.

```yaml
tests:
  - name: load balancer reaches the backend
    node: lb
    type: execute
    options:
      command: curl -sf http://{{ fact "backend" "ipv4" }}:8080/health
      evaluate:
        exit_code: 0
```

### Value Extraction and Numeric Assertions

`execute` tests can pull named values out of their output and assert on
them numerically — the building block for performance and regression
gating:

```yaml
tests:
  - name: throughput within baseline
    node: testbed
    type: execute
    options:
      command: loom run --json tcp-100g.yaml
      extract:
        throughput_mbps: { jsonpath: "$.summary.throughput_mbps" }
        p99_us:          { regex: "p99=([0-9.]+)us" }   # capture group 1
      evaluate:
        exit_code: 0
        throughput_mbps: { gte: 11852, within: 12476, tolerance_pct: 5 }
        p99_us:          { lte: 49 }
```

An `evaluate` entry whose name matches an `extract` entry takes a
comparator map instead of a standard evaluation type: `gt`/`gte`/`lt`/`lte`
(`ge`/`le` accepted), `eq`/`ne` (numeric when both sides are numbers,
string otherwise), and `within` with `tolerance_pct` or absolute
`tolerance` for baseline gating. All conditions in the map must hold.
JSON extraction reads the first JSON document in the output, so tools
that mix JSON with log lines work.

### Cross-Test Capture

Tests can record values for later tests — for assertions that compare
across a state transition (before/after a reboot, upgrade, or rollback):

```yaml
tests:
  - name: record root subvolume id
    node: iso-vm
    type: execute
    options:
      command: btrfs subvolume show / | awk '/Subvolume ID:/{print $3}'
      capture: pre_rollback_root_id     # stores trimmed stdout

  - name: rollback recreated the root subvolume
    node: iso-vm
    type: execute
    options:
      command: |
        [ "$(btrfs subvolume show / | awk '/Subvolume ID:/{print $3}')" -gt "{{capture.pre_rollback_root_id}}" ]
      evaluate:
        exit_code: 0
```

`capture:` takes a bare name (whole trimmed stdout) or a map of names to
`{jsonpath}`/`{regex}` extractors. Later tests reference values as
`{{capture.name}}` in `command`, `skip_if`, and `skip_unless`; a
reference to a value nothing captured fails the test rather than running
a mangled command. Values persist across `-i` iterations and are
overwritten by each run.

### Suite Variables and Tags

Suites parameterize with a `vars:` block, `{{var.name}}` / `{{env.NAME}}`
references (resolved at config load — a numeric var in a numeric position
stays a number), and `--vars key=value[,key=value]` CLI overrides. Capture
references and fact templates are untouched. Unresolved references are
config errors.

```yaml
suite: API smoke
vars:
  target: 10.0.0.5      # override per run: dart -c suite.yaml --vars target=192.168.1.1
tests:
  - name: api answers
    node: local
    type: http_request
    tags: [smoke, network]
    options:
      url: http://{{var.target}}:8080/health
```

Tests carry `tags:`; run subsets with `--only tag=network` (any listed tag
matches) and exclude with `--skip tag=slow`. Steps are never filtered, so
setup/teardown chains stay intact; the run reports how many tests the
filter excluded.

### Timeouts and Retries

Any `execute` test or step accepts `timeout:` (seconds; `0`/omitted means
unbounded) — a hung command fails the test with a clear `timeout` check
failure instead of hanging the suite, and teardown still runs. Note: the
remote process may keep running; the timeout bounds the suite's wait, and
a retried timeout re-awaits the same in-flight command rather than
launching another. `wait_for` differs: its `timeout` is required and must
be positive, since the step's whole purpose is to give up eventually.
`retry` is rejected on `reboot` tests — retrying one would power-cycle
the target on every failed evaluation.

Eventually-consistent assertions retry until they pass or time out:

```yaml
tests:
  - name: cluster elects a leader
    node: n1
    type: execute
    retry:
      timeout: 60      # keep retrying up to 60s
      interval: 5      # between attempts (default 2)
    options:
      command: cluster-status --leader
      timeout: 10      # per-attempt command bound
      evaluate:
        exit_code: 0
        regex: "leader=node[0-9]+"
```

`retry` reruns the test command and its evaluations (setup/teardown
commands run once); it works on every test type. The matching `wait_for`
step covers setup:

```yaml
setup:
  - name: wait for the API to answer
    node: app
    step:
      type: wait_for
      options:
        command: curl -sf http://localhost:8080/health
        timeout: 120
        interval: 3
```

### Conditional Skips

Any test can declare a skip condition — a command run on the test's node
before the test executes:

```yaml
tests:
  - name: RKE2 server is running
    node: iso-vm
    type: service_status
    skip_unless: which aether-ops-bootstrap   # nonzero exit → skip
    options:
      service: rke2-server

  - name: legacy migration check
    node: iso-vm
    type: execute
    skip_if: test -f /etc/new-style-config    # zero exit → skip
    options:
      command: legacy-migrate --verify
      evaluate:
        exit_code: 0
```

Skipped tests render with a distinct yellow `skipped` status and are
counted separately in the results (`Pass / Fail / Skip`), so a suite that
quietly skips assertions can never read as fully green. Skips do not
affect the exit code. An error running the condition command itself fails
the run — a broken condition never silently passes or skips.

---

## Test Evaluation Reference

Tests declare their pass/fail conditions in an `evaluate` block. Every listed
check must pass for the test to pass; a test with no checks is reported as
"ran" rather than passed. Checks are evaluated and reported in alphabetical
order.

### Exit code

| Check | Value | Passes when |
|-------|-------|-------------|
| `exit_code` | integer or list (`[0, 1]`) | Exit code equals the value (or is in the list) |
| `exit_code_not` | integer or list | Exit code is not the value (or not in the list) |

### Output content (stdout)

| Check | Value | Passes when |
|-------|-------|-------------|
| `match` | string | Output equals the value exactly (trailing whitespace trimmed) |
| `match` | `{value: "...", trim: false}` | Output equals the value byte-for-byte |
| `contains` | string | Output contains the substring |
| `not_contains` | string | Output does not contain the substring |
| `regex` | string | Output matches the regular expression (validated at config load) |
| `empty` | boolean | Output is empty / non-empty, ignoring whitespace |
| `line_count` | integer | Output has exactly N lines (trailing newlines ignored) |

### Stderr

| Check | Value | Passes when |
|-------|-------|-------------|
| `stderr_match` | string or `{value, trim}` map | Stderr equals the value |
| `stderr_contains` | string | Stderr contains the substring |
| `stderr_regex` | string | Stderr matches the regular expression |
| `stderr_empty` | boolean | Stderr is empty / non-empty, ignoring whitespace |

### Numeric and structured output

| Check | Value | Passes when |
|-------|-------|-------------|
| `gt` / `ge` / `lt` / `le` | number | Output, parsed as a number, satisfies the comparison |
| `json_path` | `{path: "a.b[0].c", equals: value}` | Output parsed as JSON has the expected value at the dot-path |

### Timing

| Check | Value | Passes when |
|-------|-------|-------------|
| `max_duration` | seconds (fractional allowed) | The test command completed within the bound |

Example combining several checks:

```yaml
tests:
  - name: service reports healthy
    node: localhost
    type: execute
    options:
      command: "curl -s http://localhost:8080/health"
      evaluate:
        exit_code: 0
        json_path:
          path: status
          equals: healthy
        stderr_empty: true
        max_duration: 2.5
```

---

## License

This project is distributed under an open-source or commercial license, as specified in the repository's [LICENSE](LICENSE) file.

---

*Thank you for exploring DART! Your contributions and feedback are welcome as we strive to make testing in distributed environments as seamless as possible.*
