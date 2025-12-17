# LXD/LXC Examples

This directory contains examples for using LXD containers and virtual machines as test nodes in DART.

## Prerequisites

1. **Install LXD**: Follow the [LXD installation guide](https://documentation.ubuntu.com/lxd/en/latest/installing/)
2. **Initialize LXD**: Run `lxd init` to configure LXD (accept defaults for simple setup)
3. **User permissions**: Add your user to the `lxd` group: `sudo usermod -aG lxd $USER`
4. **For VMs**: Ensure your system supports hardware virtualization: `lxc info | grep -i vm`

## Examples

### lxd.yaml - Container Example

Demonstrates LXD containers as test nodes:
- Basic container configuration
- LXD network setup
- Custom profile creation
- Multi-container tests

```yaml
nodes:
  - name: my-container
    type: lxd
    options:
      image: ubuntu:24.04
      instance_type: container  # default
      exec_opts:
        shell: /bin/bash
```

### lxd-vm.yaml - Virtual Machine Example

Demonstrates LXD virtual machines:
- Using the `lxd-vm` node type shorthand
- Using `lxd` type with `instance_type: virtual-machine`
- VM-specific considerations (boot time, kernel isolation)

```yaml
# Option 1: Use lxd-vm type
nodes:
  - name: my-vm
    type: lxd-vm
    options:
      image: ubuntu:24.04
      exec_opts:
        shell: /bin/bash

# Option 2: Use lxd type with instance_type
nodes:
  - name: my-vm
    type: lxd
    options:
      image: ubuntu:24.04
      instance_type: virtual-machine
      exec_opts:
        shell: /bin/bash
```

### lxd-remote.yaml - Remote LXD Server Example

Demonstrates connecting to remote LXD servers using modern trust token authentication or traditional certificate-based authentication:
- Trust token authentication (recommended - simple and secure)
- Certificate-based authentication (traditional method)
- Security best practices
- Mixed local/remote deployments

**Trust Token Authentication (Recommended):**

```yaml
nodes:
  - name: remote-container
    type: lxd
    options:
      # Remote server address (HTTPS)
      remote_addr: https://10.0.0.1:8443
      
      # Trust token from 'lxc config trust add'
      trust_token: eyJjbGllbnRfbmFtZSI6ImRhcnQtY2xpZW50...
      
      image: ubuntu:24.04
      instance_type: container
```

1. On the remote LXD server, enable network access and generate a token:
   ```bash
   lxc config set core.https_address "[::]:8443"
   lxc config trust add dart-client
   # Copy the generated token
   ```

2. Use the token in your DART configuration (see above).

**Certificate-Based Authentication (Traditional):**

```yaml
nodes:
  - name: remote-container
    type: lxd
    options:
      # Remote server address (HTTPS)
      remote_addr: https://10.0.0.1:8443
      
      # Client certificates for authentication
      client_cert: ~/.config/lxc/client.crt
      client_key: ~/.config/lxc/client.key
      
      # Optional: server certificate for custom CA
      # server_cert: /path/to/server.crt
      
      # Optional: skip TLS verification (NOT recommended)
      # skip_verify: false
      
      image: ubuntu:24.04
      instance_type: container
```

**Setting up remote LXD access:**

1. On the remote LXD server, enable network access:
   ```bash
   lxc config set core.https_address "[::]:8443"
   ```

2. Add the remote server and generate certificates:
   ```bash
   lxc remote add myremote https://remote-server-ip:8443
   # Follow prompts to authenticate (may require trust password)
   ```

3. Use the generated certificates located at:
   - Client cert: `~/.config/lxc/client.crt`
   - Client key: `~/.config/lxc/client.key`

4. For production environments, use proper certificate management and avoid `skip_verify: true`.


## Configuration Options

### Node Options

| Option | Description | Default |
|--------|-------------|---------|
| `image` | Image to use (format: `remote:alias`, e.g., `ubuntu:24.04`) | Required |
| `instance_type` | Type of instance: `container` or `virtual-machine` | `container` |
| `server` | Image server URL | Auto-detected from remote |
| `protocol` | Protocol: `lxd` or `simplestreams` | Auto-detected |
| `profiles` | List of profiles to apply | `["default"]` |
| `exec_opts` | Execution options (e.g., `shell: /bin/bash`) | - |
| `networks` | Network configurations | - |
| `remote_addr` | Remote LXD server HTTPS address (e.g., `https://10.0.0.1:8443`) | - |
| `trust_token` | One-time trust token from `lxc config trust add` (recommended) | - |
| `client_cert` | Path to client certificate file for remote authentication | - |
| `client_key` | Path to client key file for remote authentication | - |
| `server_cert` | Path to server certificate file (for custom CA verification) | - |
| `skip_verify` | Skip TLS verification (not recommended for production) | `false` |

### Authentication Priority

When connecting to remote LXD servers, authentication methods are tried in this order:

1. **Trust Token** (`trust_token`) - Modern, one-time token authentication (recommended)
2. **Client Certificates** (`client_cert` + `client_key`) - Traditional certificate-based authentication
3. **Skip Verification** (`skip_verify: true`) - Insecure, not recommended for production

### LXD Configuration Section

Optional section to define LXD resources managed by the test suite:

```yaml
lxd:
  networks:
    - name: test-net
      type: bridge
      subnet: 10.0.0.0/24
      gateway: 10.0.0.1
  
  profiles:
    - name: custom-profile
      description: Custom test profile
      config:
        limits.cpu: "2"
        limits.memory: "2GB"
      devices:
        root:
          type: disk
          path: /
          pool: default
```

## Common Image Sources

- `ubuntu:24.04` - Ubuntu 24.04 LTS
- `ubuntu:22.04` - Ubuntu 22.04 LTS
- `images:debian/12` - Debian 12
- `images:centos/9-Stream` - CentOS Stream 9
- `images:alpine/3.19` - Alpine Linux 3.19

List available images with: `lxc image list images:`

## Troubleshooting

**"Permission denied" error**: Ensure your user is in the `lxd` group and re-login.

**VM fails to start**: Check that your system supports hardware virtualization (`egrep -c '(vmx|svm)' /proc/cpuinfo` should return > 0).

**Image not found**: List available images with `lxc image list ubuntu:` or `lxc image list images:`.

**Slow container/VM start**: First-time image downloads can be slow. Subsequent runs use cached images.
