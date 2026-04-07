# Incus Examples

This directory contains example test suites demonstrating how to use DART with Incus containers.

## What is Incus?

Incus is a community fork of LXD that provides system containers and virtual machines. It uses the same API as LXD, which means DART can work with Incus by simply specifying the Incus socket path.

## Using Incus with DART

To use Incus instead of LXD, you have two options:

### Option 1: Configure at the Suite Level

Specify the socket in the `lxd` configuration section:

```yaml
lxd:
  socket: /var/lib/incus/unix.socket
  networks:
    - name: test-network
      subnet: 10.100.0.0/24
      gateway: 10.100.0.1
```

### Option 2: Configure at the Node Level

Specify the socket in individual node options:

```yaml
nodes:
  - name: my-container
    type: lxd
    options:
      image: ubuntu:24.04
      socket: /var/lib/incus/unix.socket
```

## Prerequisites

1. **Install Incus**: Follow the [Incus installation guide](https://linuxcontainers.org/incus/docs/main/installing/)
2. **Initialize Incus**: Run `incus admin init` to set up Incus
3. **User Permissions**: Ensure your user has access to the Incus socket (usually by being in the `incus-admin` group)

## Socket Paths

- **LXD default socket**: `/var/snap/lxd/common/lxd/unix.socket` (snap) or `/var/lib/lxd/unix.socket` (native)
- **Incus default socket**: `/var/lib/incus/unix.socket`

## Examples

- `incus.yaml` - Basic Incus container test suite with network configuration and multiple containers

## Running the Examples

```bash
# Run the basic Incus example
dart run -f examples/incus/incus.yaml
```

## Notes

- The `type: lxd` is used for both LXD and Incus nodes, as they share the same API
- All LXD features work with Incus, including:
  - Container and VM support
  - Network management
  - Profile configuration
  - Project isolation
  - Remote connections (via HTTPS)
