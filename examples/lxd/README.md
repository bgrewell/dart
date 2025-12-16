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
