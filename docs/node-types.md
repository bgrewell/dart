# Node Types

Every test and step runs on a *node*: the machine, container, or VM under test. This is the full reference for node types and their options.

[← Back to the README](../README.md)

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
      # Host keys are verified against ~/.ssh/known_hosts by default;
      # set known_hosts: <path> or insecure_skip_host_key: true to change it.
      # bastion: { host: jump.example.com, user: jumpuser, key: ~/.ssh/id_rsa }

  - name: test-container
    type: docker
    options:
      image: ubuntu:latest
      env: ["LOG_LEVEL=debug"]
      volumes: ["./fixtures:/fixtures:ro"]
      ports: ["8080:80"]
      # privileged: true          # opt-in; capabilities are usually enough
      # capabilities: [NET_ADMIN]
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

### Node Security Defaults

Two defaults changed in favour of least privilege — suites relying on the
old behaviour need one line each:

- **Docker containers are no longer privileged.** DART used to set
  `--privileged` on every container. Add `privileged: true` if a test
  genuinely needs it, or prefer `capabilities: [NET_ADMIN]` for the common
  network-testing case.
- **SSH host keys are verified.** DART used to accept any key. Verification
  uses `~/.ssh/known_hosts` by default; point `known_hosts:` elsewhere, or
  set `insecure_skip_host_key: true` for throwaway lab targets. A missing
  or unmatched key is an error naming both options.

SSH nodes also accept a `bastion:` block to reach targets through a jump
host. The bastion inherits the target's host-key policy but may set its
own (`known_hosts` / `insecure_skip_host_key`), so relaxing verification
for an ephemeral target need not relax it for the long-lived jump host;
reconnects after `reboot` route through the bastion too, and chained
bastions are rejected rather than silently dropped.

`--check` validates everything that needs no connection — SSH
credentials, `known_hosts` readability, bastion shape, and docker
`ports`/`volumes` specifications — so these breaking changes surface
before a run rather than during one. Relative volume host paths
(`./fixtures:/fixtures`) are resolved to absolute paths, since the Engine
API would otherwise treat them as *named volumes* and mount an empty one.

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
