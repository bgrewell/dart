# Multi-Node Configuration Examples

This directory contains examples demonstrating the multi-node configuration feature in DART.

## Feature Overview

The multi-node feature allows you to specify either a single node or an array of nodes for steps and tests. When an array is provided, DART automatically expands the configuration to run that step or test on each specified node.

### Syntax

**Traditional single-node syntax (still supported):**
```yaml
- name: Install package
  node: web-server
  step:
    type: apt
    options:
      packages:
        - nginx
```

**New multi-node syntax:**
```yaml
- name: Install package
  node: [web-server, app-server, db-server]
  step:
    type: apt
    options:
      packages:
        - nginx
```

The multi-node configuration above will automatically expand to three separate steps:
1. Install package on web-server
2. Install package on app-server  
3. Install package on db-server

## Benefits

- **Cleaner configurations**: Avoid repeating the same step/test definition for multiple nodes
- **Easier maintenance**: Update one configuration instead of multiple duplicate entries
- **Better readability**: Clearly see which operations run across multiple nodes

## Examples

### simulated-multi-node.yaml
A simple example using simulated steps that demonstrates the syntax without requiring actual infrastructure.

### docker-multi-node.yaml
A realistic example using Docker containers that shows how to:
- Install packages on multiple containers simultaneously
- Run the same test across multiple nodes
- Clean up resources on all nodes

## Output

When a multi-node configuration is expanded, the output will show each step/test running on its respective node. For example:

```
[+] Running test setup
  [ web-server ] Install packages ... done
  [ app-server ] Install packages ... done
  [ db-server  ] Install packages ... done
```

This makes it clear that the same operation ran on multiple nodes while keeping your configuration file concise.

## Real-World Use Cases

1. **Installing dependencies**: Install the same packages on all web servers
2. **Running health checks**: Verify service status across multiple nodes
3. **Configuration updates**: Apply the same configuration to multiple instances
4. **Cleanup operations**: Remove temporary files from all nodes in teardown
5. **Integration tests**: Test connectivity between multiple services

## Notes

- Each node in the array must be defined in the `nodes:` section of your configuration
- The expansion happens during configuration parsing, before any steps are executed
- The expanded steps/tests maintain the same order as they appear in the configuration
