# Node Types

Every test and step runs on a *node*: the machine, container, or VM under test. This is the full reference for node types and their options.

[← Back to the README](../README.md)

DART supports several types of nodes that can be used as test targets:

- **Local Node (`local`)**  
  Execute tests on the local machine where DART is running.
  Invariant: at most one `local` node per suite. A second one fails configuration
  with `only one local node allowed; "<name>" is a duplicate`, reported against
  that node's line in the YAML. The limit applies only to `local`; other types may
  appear any number of times. Several roles on one machine are modelled with a
  single local node, distinguished by test and step naming rather than by separate
  node entries.

- **Docker Node (`docker`)**  
  Run tests inside Docker containers, with volume, environment, port, capability,
  and privileged-mode options. Supports both local and remote Docker hosts.

- **Docker Compose Node (`docker-compose`)**  
  Manage and test services defined in Docker Compose files. Multiple nodes can target different services in the same compose stack.

- **LXD Node (`lxd`)**  
  Execute tests in LXD containers or virtual machines, with automatic provisioning and cleanup. Supports both local Unix socket connections and remote HTTPS connections with certificate-based authentication.

- **LXD VM Node (`lxd-vm`)**  
  Shorthand for an `lxd` node with `instance_type: virtual-machine`, identical to
  `lxd` in every other respect. Note: the alias sets `instance_type` on a copy of
  the node's options, so an explicit `instance_type` in the YAML is overridden
  rather than honoured.

- **SSH Node (`ssh`)**  
  Run tests on remote machines via SSH, supporting both key-based (`key`) and
  password (`pass`) authentication.

Each node type takes type-specific keys under `options:`. For example:

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
      key: ~/.ssh/id_rsa   # must be an unencrypted private key; see SSH Node Options
      # Host keys are verified against ~/.ssh/known_hosts by default;
      # set known_hosts: <path> or insecure_skip_host_key: true to change it.
      # bastion: { host: jump.example.com, user: jumpuser, key: ~/.ssh/id_rsa }

  - name: test-container
    type: docker
    # facts: is a sibling of options:, not one of its keys
    facts:
      kernel: uname -r
    options:
      # The image must already exist in the daemon and must run a
      # foreground process; see Docker Node Options.
      image: nginx:alpine
      env: ["LOG_LEVEL=debug"]
      volumes: ["./fixtures:/fixtures:ro"]
      ports: ["8080:80"]
      # privileged: true          # opt-in; capabilities are usually enough
      # capabilities: [NET_ADMIN]
  
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

The `lxd-vm` alias exists so a virtual machine does not need the extra option. The
two nodes below are equivalent:

```yaml
nodes:
  - name: my-vm
    type: lxd-vm          # equivalent to the block below
    options:
      image: ubuntu:24.04

  - name: my-vm-alt
    type: lxd
    options:
      image: ubuntu:24.04
      instance_type: virtual-machine
```

### Node names

The `name` of a node is more than a label — it is an identifier DART uses verbatim
on the target platform.

- **Names must be unique and non-empty.** A missing name or a repeated one is a
  configuration error reported with its file location, caught by `--check` before
  anything is created.
- **At most one `local` node per suite.** A second `local` node is rejected as a
  configuration error. This check lives in the node factory rather than the
  configuration loader, so unlike the duplicate-name check it does not surface
  under `--check`.
- **Docker nodes:** the node name becomes both the container name and the
  container's hostname. It must therefore be a legal Docker container name
  (`[a-zA-Z0-9][a-zA-Z0-9_.-]*`), and creation fails if a container of that name
  already exists — DART does not adopt or replace pre-existing containers.
- **LXD/Incus nodes:** the node name becomes the instance name, so it must satisfy
  LXD's instance-name rules (letters, digits, and hyphens; at most 63 characters)
  and must not clash with an existing instance in the target project.
- **Docker Compose nodes:** the node name is used as the Compose project name when
  `project_name` is omitted.

Note: name syntax is not validated by DART. A name the platform rejects surfaces as
the daemon's or LXD server's own error during node setup. There is no
`container_name` or `instance_name` option that decouples the platform identifier
from the node identity.

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

`--check` validates the node options that need no connection: SSH
authentication (a key file that exists and parses, or a password),
`known_hosts` readability under the configured host-key policy, and — when a
`bastion:` block is present — that it names a host, carries usable
credentials, and is not chained. For docker nodes it checks `volumes` and
`ports` specification syntax, resolving relative volume host paths
(`./fixtures:/fixtures`) to absolute paths, since the Engine API would
otherwise treat them as *named volumes* and mount an empty one. These
breaking changes therefore surface before a run rather than during one.

Note: `--check` does not verify that required fields are present. An `ssh`
node missing `host` passes the check and then fails the run dialling `:22`; a
`docker` node missing `image` fails at container creation; a `docker-compose`
node missing `compose_file` fails at node construction. Only the *bastion's*
host is required at check time.

### Unrecognised Options

Option names must match exactly. On `docker`, `docker-compose`, `ssh`, and `lxd`
nodes, `options:` is decoded by a JSON round-trip into a typed struct, so any key
that is not a recognised option is discarded without an error or a warning. A
misspelling such as `priviliged` instead of `privileged`, `hostname` instead of
`host`, or `known_host` instead of `known_hosts` leaves the option at its default
and the suite runs on.

Only `local` nodes warn. A local node prints
`Warning: node "<name>": option "<key>" is not recognized and was ignored (known options: env, shell, sudo, exec_opts)`
to stderr. That warning is specific to local nodes and is not a general guarantee.

Note: `--check` does not detect option typos on any node type. It validates the
*semantics* of recognised options that need no connection, but it decodes options
through the same round-trip that drops unknown keys, and it substitutes mock nodes
for real ones — so even the local node's warning appears only in a real run.

A dropped SSH security key fails safe: a mistyped `insecure_skip_host_key` leaves
it `false`, and a mistyped `known_hosts` falls back to `~/.ssh/known_hosts`, so
host-key verification stays on and the symptom is a confusing connection error
rather than a silent downgrade. The real cost is a silently ineffective option — a
`privileged` or `capabilities` typo, for example, surfaces later as an unexplained
permission failure inside the container.

### Node Facts

`facts:` is a top-level key on a node — a sibling of `options:`, not a key inside
it. Each entry maps a fact name to a command that runs on that node; the command's
stdout becomes the fact's value.

```yaml
nodes:
  - name: web
    type: docker
    options:
      image: nginx:alpine
    facts:
      ipaddr: "hostname -i | awk '{print $1}'"
      kernel: uname -r
```

Fact values are referenced from step options, test options, and a test's `setup:`
and `teardown:` commands as `{{ fact "web" "ipaddr" }}`, or `{{ fact "self" "ipaddr" }}`
for the node the step or test runs on. Node `options:` are not templated — facts do
not exist yet when nodes are created, so a fact reference there is left unresolved.

Semantics:

- **Timing.** Facts are gathered after node setup but before any setup step runs. A
  fact command therefore sees only what the base image or host already provides; a
  command depending on a tool installed by a setup step fails the run.
- **Order.** Within a node, fact commands run in sorted fact-name order. Nodes are
  processed in the order they appear in the configuration.
- **Failure.** A fact command that fails to execute, or exits non-zero, aborts the
  entire run before any test starts, with an error naming the fact and the node.
  Built-in address facts behave the opposite way: their discovery failures are
  ignored.
- **Output.** Trailing spaces, tabs, carriage returns, and newlines are stripped
  from stdout; leading whitespace is preserved. Only stdout is captured — stderr
  appears only in the failure message.
- **Precedence.** A fact whose name collides with a built-in network fact overrides
  the built-in.
- **Substitution is literal.** Fact values are inserted verbatim into commands with
  no shell quoting. Warning: a fact whose command can return attacker-influenced
  output can alter the command it is substituted into.
- **Reporting.** Declaring any `facts:` block turns on the "Gathering node facts"
  phase in the console output; suites that rely only on built-ins get no extra
  output.

Docker and LXD nodes also publish built-in address facts (`ipv4`, `ipv6`, and
per-network or per-interface variants such as `ipv4.test-net` or `ipv4.eth0`).
Those are documented under
[Built-in Network Facts](tests.md#built-in-network-facts). Local, SSH, and
Docker Compose nodes publish no built-in facts.

### Local Node Options

A `local` node accepts four options, all optional:

| Option | Type | Behaviour |
|---|---|---|
| `shell` | string | Shell used to run commands. Defaults to `/bin/sh` (`cmd` on Windows), so bashisms — `[[ ]]`, `source`, arrays, `pipefail` — need an explicit `shell: /bin/bash`. |
| `env` | list of `KEY=VALUE` strings | Environment for executed commands. Note: this *replaces* the inherited environment rather than adding to it, so anything the command needs — including `PATH` — must be listed. |
| `sudo` | map | Password supplied to `sudo` prompts. Either `env_var: <NAME>` (read from that environment variable when the node is constructed) or `password: <literal>`. |
| `exec_opts` | map | The same three keys nested one level, matching the shape the Docker, Compose, and LXD node types use. |

```yaml
nodes:
  - name: localhost
    type: local
    options:
      shell: /bin/bash
      env:
        - PATH=/usr/local/bin:/usr/bin:/bin
        - LOG_LEVEL=debug
      sudo:
        env_var: SUDO_PASSWORD

  # Equivalent, using the container-node nesting
  - name: localhost-nested
    type: local
    options:
      exec_opts:
        shell: /bin/bash
        sudo:
          env_var: SUDO_PASSWORD
```

Precedence and warnings:

- `env_var` wins over `password` when a `sudo` block sets both; the literal is then
  dead configuration.
- An `env_var` naming an unset or empty variable does not fail the run: DART prints
  `Warning: sudo env_var "NAME" is empty or unset` to stderr and proceeds with an
  empty password, so `sudo` typically fails later at the prompt.
- When `exec_opts` is present it becomes the sole source of exec options — top-level
  `shell`, `env`, and `sudo` are ignored. Each ignored top-level key produces a
  stderr warning; nothing fails.
- Unrecognised keys are warned about on stderr and ignored rather than rejected, so
  a typo such as `shel:` falls back to `/bin/sh`.
- A `shell`, `env`, or `sudo` value of the wrong YAML type is also warned about and
  skipped. Warning: a wrong-typed `shell` suppresses the `/bin/sh` default as well,
  leaving commands to run without a shell.

### SSH Node Options

Options for `type: ssh`, decoded by a JSON round-trip into a typed struct, so these
key names are exact:

| Key | Type | Default | Notes |
|---|---|---|---|
| `host` | string | — | Target hostname or IP address. Not separately validated; an empty value fails at dial time. |
| `port` | int | `22` | TCP port of the SSH service. |
| `user` | string | — | Remote username; no default. |
| `key` | string | — | Path to an unencrypted private key file. A leading `~` is expanded. |
| `pass` | string | — | Password authentication. The key is `pass` — not `password`. |
| `known_hosts` | string | `~/.ssh/known_hosts` | OpenSSH `known_hosts` file used for verification; `~` is expanded. |
| `insecure_skip_host_key` | bool | `false` | Opts out of host-key verification entirely. |
| `bastion` | map | — | Jump host; see below. |

At least one of `key` or `pass` must be set. Both may be set, in which case the
public key is offered first and the password second. With neither, node
construction fails with:

```text
no ssh credentials configured: set key or pass
```

and `--check` reports the same error without connecting.

```yaml
nodes:
  - name: remote-server
    type: ssh
    options:
      host: example.com
      port: 22
      user: testuser
      pass: hunter2          # or key: ~/.ssh/id_rsa; at least one is required
```

Note: `key:` must point at an unencrypted private key. DART parses it with
`ssh.ParsePrivateKey`, which has no passphrase variant, so a passphrase-protected
key fails with
`unable to parse private key: ssh: this private key is passphrase protected`.
There is no passphrase option, and ssh-agent (`SSH_AUTH_SOCK`) is not consulted.
Adding `pass:` does not help: a key that fails to parse aborts node construction
before password authentication is attempted — use `pass:` on its own for password
authentication, or generate a dedicated unencrypted key for test runs. The same
applies to `key:` inside a `bastion:` block. `--check` catches an unusable key
before any connection is made.

A leading `~` is expanded to the current user's home directory in `key` and
`known_hosts`, on both the node and its `bastion:`. `~otheruser/...` is not
resolved; another user's home needs an absolute path.

#### Bastion options

A `bastion:` block accepts the same keys as the target: `host`, `port` (default
`22`), `user`, `key`, `pass`, `known_hosts`, and `insecure_skip_host_key`.

```yaml
      # bastion:
      #   host: jump.example.com
      #   port: 22
      #   user: jumpuser
      #   key: ~/.ssh/id_rsa      # or pass: <password>
      #   known_hosts: ~/.ssh/known_hosts_jump   # optional; inherits the target's otherwise
      #   insecure_skip_host_key: false          # optional; inherits the target's otherwise
```

Differences from the target node:

- `host` is required; omitting it fails with `bastion host is required`.
- `known_hosts` and `insecure_skip_host_key` are inherited from the target node
  when unset and override it when set. `insecure_skip_host_key` is tri-state on the
  bastion for that reason.
- A nested `bastion` inside `bastion` is rejected with
  `chained bastions are not supported: remove the nested bastion block`.
- The bastion needs its own credentials; missing ones fail with
  `bastion: no ssh credentials configured: set key or pass`.

#### SSH connection lifecycle

An `ssh` node connects when it is constructed — during dependency wiring, after the
configuration has been loaded and validated, but before platform setup, before node
setup, and before any setup step runs. Consequences worth knowing:

- **Every ssh node must be reachable at startup, on every run.** If any one of them
  cannot be reached, or its host key cannot be verified, DART reports the error and
  exits before executing anything. This includes `--teardown-only` runs: a suite
  cannot be torn down through DART once one of its ssh targets is down. `--check`
  validates credentials, `known_hosts` readability, and bastion shape without
  connecting.
- **`Setup` and `Teardown` do nothing on an ssh node.** The `setting up node` and
  `tearing down node` lines are no-ops; there is nothing to provision or destroy.
  Preparation and cleanup on an ssh target belong in `setup:` and `teardown:`
  steps.
- **One connection is reused for the whole run**, and is closed when the process
  exits, along with the bastion tunnel. If the target reboots or drops the
  connection outside DART's `reboot` step, the client goes stale and subsequent
  commands fail; only the `reboot` step redials, through the bastion when one is
  configured. An expected restart is best modelled with a `reboot` step rather than
  triggered from an `execute` step.

### Docker Node Options

| Option | Type | Notes |
|---|---|---|
| `image` | string | Image reference used to create the container. Must already exist in the daemon. |
| `env` | list of `KEY=VALUE` strings | Environment set at container creation. |
| `volumes` | list of `host:container[:options]` | Bind mounts; relative host paths are resolved to absolute paths and a leading `~` is expanded. |
| `ports` | list of `host:container[/proto]` | Published ports. |
| `privileged` | bool | Opt-in full host capabilities; defaults to `false`. |
| `capabilities` | list of strings | Individual Linux capabilities, for example `[NET_ADMIN]`. |

Note: DART does not pull Docker images. The `image:` a docker node references must
already exist in the local daemon — pulled beforehand (`docker pull nginx:alpine`)
or built from the suite's `docker.images` block, which runs `docker build` against
a Dockerfile and is therefore not a substitute for a pull. A missing image fails
node setup with `could not create container: ...` followed by the daemon's
`No such image`. This applies to `type: docker` nodes only: `docker-compose` nodes
pull through Compose, and LXD/Incus nodes fetch images through the LXD client.

Note: the container is created from the image's own `CMD`/`ENTRYPOINT`. DART sets
the image, hostname, environment, published ports, bind mounts, and the privilege
options from the table above, and offers no `command`,
`entrypoint`, or `tty` option, so the image must run a process that stays in the
foreground. After starting the container, node setup polls every second for up to
two minutes until the container reports `Running` and a trivial `exec` of `true`
succeeds. An image whose `CMD` exits immediately — such as bare `ubuntu:latest`,
whose `CMD` is `/bin/bash` and which exits at once because DART allocates no TTY
and attaches no stdin — never becomes ready, and setup fails after two minutes with
`container <name> not ready: timeout waiting for container ... context deadline exceeded`.
A service image (`nginx:alpine`, `postgres:16`) or a purpose-built image whose
`CMD` is a supervisor satisfies the check; `examples/docker/docker.yaml` builds
exactly such an image through the `docker.images` block.

Warning: `networks` on a `docker` node is not implemented. The option parses but is
never applied — `DockerNode.Setup` does not read it, and containers are created
with an empty networking configuration and no network mode, so they attach to
Docker's default bridge and the `subnet`/`ip` values have no effect. Platform-level
`docker.networks` still creates and removes the named networks, but no container
joins them. Because the default bridge has no embedded DNS, containers cannot
resolve each other by node name; `{{ fact "<node>" "ipv4" }}` yields a container's
actual bridge address. Node-level `networks` is implemented for `lxd` nodes only.

Note: Docker and Docker Compose nodes always run commands as `sh -c "<command>"`
inside the target container; the shell is not configurable. The `exec_opts` block
honoured by `lxd` and `local` nodes is parsed but ignored on `docker` and
`docker-compose` nodes, and no warning is emitted, so `exec_opts: {shell: /bin/bash}`
silently has no effect. Commands must therefore be POSIX-sh compatible — no
bashisms such as `[[ ]]`, `<<<`, or arrays — or invoke the interpreter explicitly,
for example `bash -c '...'`. Container environment variables are set at creation
time with the docker node's `env:` option rather than at exec time;
`docker-compose` nodes take their environment from the compose file. `sudo` is
likewise unavailable as an exec option: commands run as the image's exec user, and
a docker node needing extra privileges uses `capabilities:` or `privileged:`.

### Docker Platform Configuration

The optional top-level `docker:` block declares networks and images that are
created before node setup and removed during platform teardown. It is consulted
only when present.

```yaml
docker:
  networks:
    - name: test_net              # network name passed to the Docker API
      subnet: 192.168.200.0/24    # IPAM subnet
      gateway: 192.168.200.1      # IPAM gateway
  images:
    - name: test_server           # image repository name
      tag: latest                 # image tag
      dockerfile: dockerfiles/server.dockerfile
```

- **Path resolution.** A relative `dockerfile` is joined to the directory of the
  suite YAML file, not to the process working directory. Absolute paths are used
  as-is.
- **Build command and context.** DART splits the resolved Dockerfile path into a
  directory and a filename and runs `docker build -t <name>:<tag> -f <filename> .`
  through a shell with the working directory set to the Dockerfile's own directory.
  The build context is therefore that directory, so `COPY` and `ADD` sources must
  be relative to it rather than to the suite file.
- **Lifecycle.** Networks are created and images built during platform setup,
  before node setup. During platform teardown every listed network is removed and
  then every listed image is removed. Resources that are already gone are tolerated.

Warning: teardown is destructive and name-based rather than ownership-based. DART
removes any network or image matching the configured name, whether or not this run
created it, so a pre-existing image or network sharing a name in `docker:` is
deleted. Image removal also passes only `name` with no tag, which Docker resolves
as `<name>:latest`: with a non-`latest` `tag`, the image DART just built is left
behind while an unrelated `<name>:latest` is deleted instead. Suite-unique names
are recommended, and `tag: latest` until the tag handling is fixed.

Note: node-level `networks` is inert on docker nodes (see Docker Node Options), so
the networks declared here are created and removed but joined by nothing.

See `examples/docker/docker.yaml` for a complete worked example.

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

Warning: `volumes` host paths are resolved on the machine running DART but
interpreted by the daemon. DART expands a leading `~` from the local `$HOME` and
makes any relative path absolute against DART's working directory; the result is
handed to the daemon as-is. With a remote `DOCKER_HOST`, `./fixtures:/fixtures`
becomes a local absolute path the daemon host probably does not have — and a bind
source that does not exist is created as an empty directory rather than failing, so
a test can read nothing and still pass. `--check` validates the
`host:container[:options]` shape only; it cannot know what exists on the daemon
host. Remote daemons need paths that exist on the daemon host, a named volume (a
bare name with no `/` or `.`, which DART leaves untouched for the daemon to
resolve), or fixtures copied in with a `file_copy` step instead of a bind mount.

See `examples/docker/docker-remote.yaml` for a complete example.

### Docker Compose Teardown

Compose stacks are torn down only by the process that started them. A
`docker-compose` node records its stack during setup, and `Teardown` is a no-op
when that record is absent. A `--teardown-only` run is a fresh process that never
runs node setup, so it does not issue `docker compose down` for the stack — and,
because it reports no error, the run still shows node teardown as completed. For
the same reason, teardown *steps* targeting a `docker-compose` node fail in that
mode with `compose stack not initialized`, since there is no running stack handle
to exec into.

Note: Docker and LXD nodes are unaffected. They address the container or instance
by name and treat "not found" as already cleaned up.

A stack left behind by an aborted run is cleaned with the same command DART would
have issued, from the directory containing the compose file:

```bash
docker compose -f <compose_file> -p <project_name> down
```

`project_name` defaults to the node's name when the option is omitted.

### LXD Node Options

| Option | Type | Default | Notes |
|---|---|---|---|
| `image` | string | — | Image reference; see [LXD/Incus Auto-Detection](#lxdincus-auto-detection) for the recognised remotes. |
| `empty` | bool | `false` | Create the instance with no source, so it boots from its devices. |
| `instance_type` | string | `container` | `container` or `virtual-machine`. |
| `profiles` | list of strings | LXD's `default` profile | LXD profiles applied to the instance. |
| `config` | map | — | Instance configuration keys, applied at creation. |
| `devices` | map | — | Arbitrary LXD device configuration, merged over the NICs generated from `networks`. |
| `networks` | list of `{name, ip}` | — | NIC devices attaching the instance to LXD networks. |
| `boot_wait` | map | — | Replaces the default readiness check; see [Empty VMs and ISO Boot](#empty-vms-and-iso-boot). |
| `exec_opts` | map | — | Currently one key, `shell`, defaulting to `/bin/bash`. |
| `project` | string | `default` | LXD project the instance is created in. Not inherited from `lxd.project`. |
| `socket` | string | auto-detected | Unix socket path; used only when the suite has no top-level `lxd:` block. |
| `server`, `protocol` | string | `local`, `lxd` | Image server URL and protocol; used only with a bare image alias. |
| `remote_addr`, `trust_token`, `client_cert`, `client_key`, `server_cert`, `skip_verify` | — | — | Remote connection settings; used only when the suite has no top-level `lxd:` block. See [Remote LXD Support](#remote-lxd-support). |

#### `networks`

`networks:` is a list of `{name, ip}` entries. Each entry becomes a NIC device on
the instance, named `eth0`, `eth1`, … in list order, with `type: nic` and
`network:` set to the entry's `name`. Because the names follow the conventional
`ethN` sequence, an entry replaces the profile's NIC of the same name — this is how
an lxd node attaches to a network declared under the suite's `lxd.networks:` block.

`ip` is optional. When given it is parsed and set as `ipv4.address` or
`ipv6.address` according to the address family; a value that does not parse as an
IP address fails node setup with
`invalid IP address for network <name>: <value>`.

```yaml
nodes:
  - name: test-container
    type: lxd
    options:
      image: ubuntu:24.04
      instance_type: container
      networks:
        - name: test-network
          ip: 10.100.0.10
```

Note: an entry also accepts a `subnet` field, but nothing reads it. The subnet
belongs on `lxd.networks[].subnet`, which is what actually creates the bridge.

The generated NICs are applied first, and any same-named key under `devices`
overwrites them, which is what makes `devices` usable for overriding a generated
NIC.

#### `profiles`

`profiles:` is a list of LXD profile names applied to the instance. Omitting it
leaves the instance with LXD's own `default` profile.

Warning: the list *replaces* the instance's profiles rather than adding to them.
Naming any profile drops `default`, and with it the root disk and `eth0` that
`default` normally supplies — the instance then fails to create, or comes up with
no storage and no network. Include `default` explicitly unless the named profiles
provide those devices themselves.

This is the only way to apply a profile declared under the suite's `lxd.profiles:`
block: DART creates those profiles during platform setup but attaches them to
nothing, so a profile that is not named here is created, left unused, and deleted
at teardown.

#### `exec_opts`

`exec_opts.shell` sets the shell DART runs commands through inside the instance and
defaults to `/bin/bash`. Every command is executed as `<shell> -c "<command>"`,
which covers test and step commands, `boot_wait.ready_command`, the `ready_command`
of a `reboot` step or test, and the readiness poll after a snapshot restore.

```yaml
nodes:
  - name: alpine-container
    type: lxd
    options:
      image: images:alpine/3.20
      exec_opts:
        shell: /bin/sh      # default: /bin/bash
```

Images without bash — alpine, busybox, most minimal images, and an empty VM before
the install has landed — must set `shell:` to an interpreter that exists in the
image, otherwise every command and every readiness poll fails.

Note: with `boot_wait` configured but no `ready_command`, DART still polls
`<shell> -c true`, so a missing shell blocks readiness even when no command was
supplied. A missing shell surfaces as a `boot_wait` timeout naming the full argument
vector, since the per-poll exec error is not reported; a plain command failure
surfaces as an `error executing command` from LXD.

Note: `exec_opts.shell` is honoured by `lxd` nodes only. Docker nodes always exec
through `sh -c` and ignore `exec_opts` entirely.

### Remote LXD Support

LXD nodes support remote connections using modern trust token authentication or traditional certificate-based authentication.

Warning: node-level remote LXD options only apply when the configuration has no
top-level `lxd:` block.

When a suite defines an `lxd:` block, DART builds a shared LXD platform wrapper.
That wrapper connects over a local Unix socket only — either the path in
`lxd.socket` or the auto-detected LXD/Incus socket. The `lxd:` block has no
remote-connection fields; it accepts only `socket`, `project`, `networks`,
`profiles`, and `images`.

Once that wrapper exists, every `lxd` and `lxd-vm` node is constructed through it
and reuses its local connection. The node options `remote_addr`, `trust_token`,
`client_cert`, `client_key`, `server_cert`, `skip_verify`, and `socket` are then
ignored — no warning is emitted, `--check` does not flag it, and instances are
created on the local host instead of the remote server.

Targeting a remote LXD server therefore means omitting the top-level `lxd:` block
entirely and configuring the connection per node. The project, network, and profile
management features described under [LXD Project Support](#lxd-project-support) are
local-socket only and cannot be combined with remote nodes.
`examples/lxd/lxd-remote.yaml` omits the `lxd:` block for this reason.

**Trust Token Authentication (Recommended):**

Configure the remote LXD server and generate a trust token:

```bash
# On the remote LXD server
lxc config set core.https_address "[::]:8443"
lxc config trust add dart-client
# Copy the generated token
```

The token goes in the node's options:

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

The generated certificates go in the node's options:

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
        # Detach the installer media once the install powers the VM off,
        # then boot from disk
        eject_on_poweroff: [iso]
```

Notes:

- `empty: true` creates the instance with no source. Omitting `image` has the same effect;
  setting both `empty: true` and `image` is rejected.
- `devices` accepts any LXD device configuration and is merged over the NICs generated from
  [`networks`](#networks), so a node can override a generated device if it needs to.
- Relative `source` paths on pool-less disk devices are made absolute against DART's working
  directory — not the suite file's directory, unlike `docker.images[].dockerfile`. A disk
  device that names a `pool` refers to a storage volume and is passed through untouched, as
  are all sources on remote nodes, which are paths on the remote server.
- `boot_wait` replaces the default readiness check: DART polls `ready_command` through the
  node's shell (`exec_opts.shell`, default `/bin/bash`) until it exits zero or the timeout
  expires. Without `ready_command`, being able to run any command at all counts as ready. An
  installer image without bash must set `exec_opts.shell` accordingly.
- `boot_wait.eject_on_poweroff` names devices to detach when the instance powers itself off
  during the boot wait. An unattended install that ends by powering off — for example
  autoinstall's `shutdown: poweroff` — leaves the ISO attached at a higher `boot.priority`
  than the root disk, so every subsequent boot would run the installer again and the readiness
  poll would never succeed. With this set, DART waits after `initial_delay` for the instance to
  reach the `Stopped` state, polling at `interval` and bounded by `timeout`, detaches each
  named device, starts the instance again, and only then begins polling `ready_command`. The
  wait for poweroff and the readiness poll each get the full `timeout`, so the worst-case setup
  time for such a node is roughly twice `timeout`. A device name that is not present on the
  instance fails node setup with
  `device "<name>" in eject_on_poweroff not found on instance <node>`; the names must match the
  keys of the node's own `devices:` map. A device inherited from a profile is not in the
  instance's local device map and is rejected as not found, so install media that must be
  ejected has to be declared on the node rather than on a profile.
- `timeout`, `interval`, and `ready_command` are not creation-only: the same readiness poll
  runs again after a `reboot` step or test on this node — where the step's own `ready_command`
  and `timeout` take precedence, but `interval` still comes from `boot_wait` — and after a
  snapshot restore, which has no per-step override. A long `timeout` chosen for an ISO install
  is therefore also the ceiling for every later reboot and restore, while a node with no
  `boot_wait` gets the five-minute and two-second defaults and a bare readiness check.
  `initial_delay` and `eject_on_poweroff` apply only at creation.
- An empty instance with no `boot_wait` is started and left alone, since it has no guest agent
  to wait for.

See `examples/lxd/lxd-iso-vm.yaml` for a complete example.

### Node Readiness

Node setup blocks until the target accepts commands. The bounds differ by node type
and are not the same as the `boot_wait` values above.

- **Docker nodes.** After the container is started, DART blocks until it reports
  `Running` and `sh -c true` exits zero inside it, polling every second for up to
  two minutes; failing to reach that state fails node setup with
  `container <name> not ready`. The bound is fixed — no YAML option changes the
  docker timeout or poll interval. A container whose image has no `sh` never
  satisfies the check.
- **LXD nodes, default path.** Unless `boot_wait` is set, DART polls every two
  seconds for up to five minutes until all three conditions hold: instance status is
  `Running`; at least one interface reports an address with **global** scope, so
  loopback and link-local addresses do not count; and `true` executes successfully
  through the guest agent. Because the address check must pass, an instance attached
  only to a network with no address assignment — or with no NIC at all — never
  becomes ready and fails after the full five minutes with
  `timeout waiting for instance <name> to become ready`.
- **`boot_wait` replaces that check entirely**, including the running-status and
  global-address requirements: DART only polls `<shell> -c <ready_command>`
  (defaulting to `true`) until it exits zero. That is what makes it usable for an
  instance that is unreachable mid-install, and it is also the way to opt out of the
  address requirement or to change the five-minute bound (`timeout`) and two-second
  poll (`interval`) — those two `boot_wait` defaults are the same values the default
  path uses.
- **An `empty: true` instance with no `boot_wait`** skips readiness entirely and is
  started and left alone.

Local and SSH nodes have no readiness wait; their `Setup` is a no-op.

### LXD/Incus Auto-Detection

DART automatically detects whether the host system has LXD or Incus installed and configures the appropriate socket path. This allows test configurations to be portable across systems without modification.

**Detection Priority:**
1. `/var/lib/incus/unix.socket` (Incus)
2. `/var/snap/lxd/common/lxd/unix.socket` (LXD snap)
3. `/var/lib/lxd/unix.socket` (LXD native)

**Explicit socket paths:**

`socket:` can be set at the suite level (`lxd.socket`) or per node
(`options.socket`). A node-level `socket` is used only when the suite has no `lxd:`
block; with an `lxd:` block present, every LXD node connects through the platform
wrapper and its own `socket` value is ignored.

When a socket path is supplied explicitly, the runtime is inferred by exact string
match: `/var/lib/incus/unix.socket` is treated as Incus, and every other path is
treated as LXD. A custom, rootless, or bind-mounted Incus socket is therefore
classified as LXD and image names are not translated — `ubuntu:24.04` is looked up
unchanged against the LXD `ubuntu` remote. Incus-native image references such as
`images:ubuntu/24.04/cloud` avoid the problem with non-standard socket paths.
Auto-detection, used when neither `socket` nor `remote_addr` is set, probes the
three listed paths and does classify the runtime correctly.

**Image Name Translation:**

When Incus is detected, DART rewrites image references that contain a `:`.
References without a colon are passed through untouched, as are all references when
the runtime is LXD. A reference of the form `<remote>:<alias>` becomes
`images:<remote>/<alias>/cloud`, selecting the cloud variant:

- `ubuntu:24.04` becomes `images:ubuntu/24.04/cloud`
- `images:debian/12` remains unchanged (references already on the `images` remote
  are never rewritten)

Warning: the rewrite applies to every remote prefix, not just `ubuntu`. A reference
such as `lxc:alpine/3.18` becomes `images:lxc/alpine/3.18/cloud`, which is not a
real alias. Non-`ubuntu` remotes should be written in Incus-native form
(`images:alpine/3.18`) so they pass through unchanged.

**Image sources:**

The image remotes DART understands are a closed set of three aliases:

| Prefix | Server | Protocol |
|---|---|---|
| `ubuntu:` | `https://cloud-images.ubuntu.com/releases` | `simplestreams` |
| `images:` | `https://images.linuxcontainers.org` | `simplestreams` |
| `lxc:` | `https://images.linuxcontainers.org` | `simplestreams` |

These are not `lxc remote` names: a remote added with `lxc remote add` is unknown to
DART.

- On an LXD host, and on any remote LXD connection via `remote_addr`, an `image:`
  whose prefix is not one of those three fails node construction with
  `unknown image server alias: <prefix>` before any instance is created. On an Incus
  host the reference is rewritten first, so an unrecognised prefix is not rejected up
  front — it turns into an `images:` lookup that fails later as image-not-found.
- An `image` with no colon is not expanded to any remote. It is used as-is with the
  node's `server` and `protocol` options, which default to `local` and `lxd`, so the
  alias resolves against the connected LXD/Incus server rather than a public image
  server.
- `server:` (an image server URL, such as a private simplestreams mirror) and
  `protocol:` (`lxd` or `simplestreams`) are the escape hatch for other image
  servers. Both are overwritten whenever `image` contains a `remote:alias` prefix,
  so they take effect only with a bare alias.

**Limitations:**

This auto-detection provides basic compatibility but has limitations. Production suites and complex scenarios are better served by configuring test definitions explicitly for the target virtualization platform:

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
- **Scoped Cleanup**: Teardown removes the profiles and networks DART created for the project, then the project itself
- **Multi-tenancy**: Run multiple test suites in parallel

Note: the `lxd:` platform block connects over a local Unix socket only. Projects,
networks, and profiles cannot be created on a remote LXD server, and adding this
block causes any node-level `remote_addr` or `trust_token` settings to be ignored.
See [Remote LXD Support](#remote-lxd-support).

**Configuring a Project:**

```yaml
lxd:
  project:
    name: dart-test-project          # required
    description: Test project        # optional
    config:                          # optional; these four default to "true"
      features.images: "true"
      features.profiles: "true"
      features.networks: "true"
      features.storage.volumes: "true"
      # e.g. features.networks: "false" to share the default project's networks

  # Networks are created within the project, always as bridges
  networks:
    - name: test-network
      subnet: 10.100.0.0/24   # required; must be valid CIDR notation
      gateway: 10.100.0.1     # required; must be a valid IP address
      nat: true               # optional; defaults to true when omitted

  # Profiles are created within the project
  # The default profile is automatically copied
  profiles:
    - name: custom-profile
      description: Custom profile for tests
      config:
        limits.cpu: "2"
        limits.memory: "2GB"
      # Devices attached to every instance using the profile
      devices:
        root:
          type: disk          # required; the only key always sent to LXD
          path: /
          pool: default

nodes:
  - name: test-container
    type: lxd
    options:
      # Required: instances are NOT placed in lxd.project automatically
      project: dart-test-project
      image: ubuntu:24.04
      instance_type: container
      profiles: [default, custom-profile]
      networks:
        - name: test-network
```

**Project configuration:**

- The whole `lxd.project` block is optional. When present, `name` is the only
  required field — an empty name fails platform setup with
  `project name cannot be empty`. `description` and `config` are optional and are
  not validated.
- All four `features.*` keys in the example are DART's own defaults rather than
  requirements. They are injected as `"true"` only when absent from `config`, so
  omitting `config:` entirely produces an identical project. Values are plain
  strings and must be quoted in YAML.
- Any value supplied for one of those keys is passed through unchanged — setting
  `features.networks: "false"`, for example, makes the project inherit that resource
  class from LXD's `default` project instead of owning its own copy. Other LXD
  project configuration keys may also be set; DART forwards the map verbatim and
  only fills in the four defaults.

**Network fields:**

- Every `lxd.networks[]` entry is created as an LXD **bridge** network. A `type:`
  key is accepted by the configuration parser but never read, so writing `type: ovn`
  silently produces a bridge with no warning.
- `subnet` must be valid CIDR notation and `gateway` must be a valid IP address.
  Both are validated locally before any request reaches the LXD server, so a
  malformed value fails platform setup with
  `network <name>: subnet "<value>" is not valid CIDR notation` or
  `network <name>: gateway "<value>" is not a valid IP address`. The bridge address
  is composed from `gateway` plus the prefix length taken from `subnet`.
- `nat` defaults to `true` when omitted and sets both `ipv4.nat` and `ipv6.nat` on
  the bridge. `nat: false` yields an air-gapped bridge: instances still receive
  addresses from the bridge and can reach each other and the gateway, but have no
  NATed route off the host. Both families must be closed together, because LXD
  auto-assigns an IPv6 subnet with NAT enabled — disabling only IPv4 NAT would leave
  outbound IPv6 working. DART does not set `ipv6.address`, so LXD still auto-assigns
  an IPv6 subnet; only NAT is disabled.
- Only the IPv4 subnet and gateway are configurable. There is no field for an IPv6
  subnet, DNS, or other bridge options.

**Profile devices:**

- Profile devices are keyed by device name. Only `type`, `path`, `pool`, and `name`
  are first-class keys. `type` is always sent to LXD; `path`, `pool`, and `name` are
  sent only when non-empty.
- Every other LXD device key — `source`, `nictype`, `parent`, `boot.priority`, and
  so on — must be nested under `opts:`:

  ```yaml
        devices:
          eth0:
            type: nic
            name: eth0
            opts:
              nictype: bridged
              parent: test-network
  ```

  Values under `opts:` and under a profile's `config:` are decoded as strings, so
  numeric and boolean values must be quoted (`boot.priority: "10"`, `limits.cpu: "2"`);
  an unquoted number fails configuration loading with a YAML type error.

  Keys placed at the top level of a profile device are discarded by the YAML parser,
  so a device copied from the node-level flat form loses everything except those
  four keys.
- `opts` entries are merged into the device map after the first-class keys, so an
  `opts` entry named `type`, `path`, `pool`, or `name` overrides the first-class
  value.
- Warning: profile devices behave differently from node-level `options.devices`,
  where the device map is flat and accepts arbitrary LXD keys directly, a missing
  `type` is reported as an error, and a relative `source` on a pool-less disk device
  is resolved to an absolute path. Profile devices get none of those three
  behaviours — an empty `type` is passed to LXD unchecked, and `opts.source` is sent
  verbatim, so profile disk sources must be absolute paths on the LXD host.

**Important Notes:**

- The default profile is automatically copied to new projects.
- Projects are created during setup and deleted during teardown.
- Each `lxd` node must set `project:` explicitly to be created inside
  `lxd.project`. Omitting it makes the node use LXD's `default` project: an unset
  `project` defaults to the literal `"default"`, and the node's LXD client is
  captured when the node is constructed — before the LXD platform setup switches the
  wrapper's server to the configured project. The instance is therefore created in
  `default` even though `lxd.networks` and `lxd.profiles` were created inside the
  named project. A node that then references one of those project-scoped networks or
  profiles fails, and project teardown, which only enumerates instances inside the
  named project, does not see the stray instance.
- A profile declared under `lxd.profiles` is attached to nothing unless a node names
  it in `profiles:`; likewise a network under `lxd.networks` is joined only by nodes
  that name it in `networks:`.
- Project deletion does not cascade. Teardown removes the profiles listed under
  `lxd.profiles` (the `default` profile is never deleted), then the networks listed
  under `lxd.networks`, then the project. Resources that no longer exist are treated
  as already removed.
- Instances are removed by node teardown, not by project deletion. If any instance
  still exists in the project, teardown fails with
  `project <name> still contains <N> instance(s), cannot delete`, and the project —
  along with anything DART did not explicitly create — is left in place.
- In a normal run a failing node teardown aborts the ordered teardown sequence. The
  error-cleanup path then retries node teardown best-effort and still attempts
  platform teardown, so an instance that could not be removed surfaces as
  `Error cleaning up lxd environment: project <name> still contains ...` and leaves
  the project behind. With `--teardown-only` the project is not deleted at all,
  because the project name is recorded only during setup; only the configured
  profiles and networks are removed.
- Resources DART did not create inside the project — extra instances, networks,
  profiles, storage volumes — are never removed.
- Note: the `lxd:` block also accepts an `images:` key (`alias`, `server`,
  `protocol`), but it is unimplemented. The LXD platform manager reads only
  `socket`, `project`, `networks`, and `profiles`, so anything under `lxd.images` is
  parsed and then ignored without a warning. Image selection is per node, via the
  node's `image:` option.

See `examples/lxd/lxd-project.yaml` for a complete example.
