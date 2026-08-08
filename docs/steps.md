# Setup and Teardown Steps

Steps prepare and clean up the environment around your tests. Unlike tests they have no pass/fail conditions — a step either completes or fails the run.

[← Back to the README](../README.md)

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
