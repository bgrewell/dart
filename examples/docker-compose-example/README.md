# Docker Compose Node Type

This example demonstrates how to use the `docker-compose` node type in DART to test applications defined in Docker Compose files.

## Overview

The `docker-compose` node type allows you to:
- Start and manage Docker Compose stacks as part of your test suite
- Execute commands in specific services within a compose stack
- Test multiple services in the same compose stack by defining multiple nodes
- Share compose stacks efficiently across multiple nodes

## Key Features

### Multiple Services Support

A Docker Compose file typically defines multiple services (containers). DART handles this by allowing you to define **multiple nodes** that all point to the same compose file, with each node targeting a different service.

This maintains the clean 1:1 relationship between nodes and execution targets that DART uses throughout.

### Efficient Stack Management

When multiple nodes reference the same compose file and project name:
- The stack is only started once (on first node setup)
- All nodes share the same running stack
- The stack is only torn down when the last node is torn down

This prevents duplicate containers and ensures efficient resource usage.

## Configuration

### Basic Node Configuration

```yaml
nodes:
  - name: web-node
    type: docker-compose
    options:
      compose_file: docker-compose.yml  # Path to compose file (required)
      project_name: my-project          # Compose project name (optional, defaults to node name)
      service: web                      # Service to target for this node (required)
```

### Required Options

- **compose_file**: Path to the docker-compose.yml file
- **service**: The name of the service within the compose file that this node should target

### Optional Options

- **project_name**: Docker Compose project name. If not specified, defaults to the node name. Multiple nodes with the same `compose_file` and `project_name` will share the same stack.
- **exec_opts**: Additional execution options (similar to other node types)

## Example Usage

### Docker Compose File (docker-compose.yml)

```yaml
version: '3.8'

services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    networks:
      - app-network

  db:
    image: postgres:alpine
    environment:
      POSTGRES_PASSWORD: example
      POSTGRES_USER: testuser
      POSTGRES_DB: testdb
    networks:
      - app-network

networks:
  app-network:
    driver: bridge
```

### DART Configuration (config.yaml)

```yaml
---
suite: Docker Compose Example

nodes:
  - name: localhost
    type: local
    options:
      shell: /bin/bash
  
  # Node targeting the web service
  - name: web-node
    type: docker-compose
    options:
      compose_file: docker-compose.yml
      project_name: dart-test
      service: web
  
  # Node targeting the db service (shares the same stack)
  - name: db-node
    type: docker-compose
    options:
      compose_file: docker-compose.yml
      project_name: dart-test  # Same project name = shared stack
      service: db

tests:
  # Test targeting the web service
  - name: check hostname on web service
    node: web-node
    type: execute
    options:
      command: "hostname"
      evaluate:
        exit_code: 0

  # Test targeting the db service
  - name: verify postgres is running on db service
    node: db-node
    type: execute
    options:
      command: "ps aux | grep postgres"
      evaluate:
        exit_code: 0
```

## How It Works

1. **Setup Phase**: When the first node referencing a compose stack is set up, DART runs `docker compose up -d` to start all services defined in the compose file.

2. **Test Execution**: When a test runs on a docker-compose node, DART executes the command inside the specific service container that the node targets.

3. **Teardown Phase**: When the last node referencing a compose stack is torn down, DART runs `docker compose down` to stop and remove all containers, networks, and volumes.

## Best Practices

1. **Use descriptive node names**: Name your nodes after the service they target (e.g., `web-node`, `db-node`) for clarity.

2. **Share stacks when appropriate**: Use the same `project_name` for nodes that should share a compose stack.

3. **One service per node**: Each node should target exactly one service. This maintains the clean abstraction and makes tests easier to understand.

4. **Wait for services**: Add setup steps to wait for services to be fully ready before running tests:
   ```yaml
   setup:
     - name: wait for services
       node: localhost
       step:
         type: simulated
         options:
           time: 5
   ```

## Differences from Regular Docker Nodes

| Feature | Docker Node | Docker Compose Node |
|---------|-------------|---------------------|
| Container management | Single container | Multiple containers in a stack |
| Configuration | Image, networks, etc. | Compose file path |
| Service targeting | N/A - single container | Required - specify which service |
| Shared resources | Independent | Can share stacks across nodes |
| Lifecycle | Per-node | Shared across nodes with same stack |

## Requirements

- Docker with built-in `docker compose` command (not the older `docker-compose` standalone tool)
- Docker Compose file version 3.0 or higher recommended
- Docker daemon accessible from DART

## Running the Example

```bash
# From the example directory
cd examples/docker-compose-example

# Run the tests
dart -c config.yaml
```

## Troubleshooting

### "docker compose command not found"

Make sure you're using a recent version of Docker that includes the built-in `docker compose` command (not the older `docker-compose` tool).

### "service 'xyz' not found in compose stack"

Verify that:
1. The service name in your node configuration matches the service name in your docker-compose.yml
2. The compose stack started successfully (check docker logs)
3. The service labels are correctly set by Docker Compose

### Stack not tearing down

If multiple nodes reference the same stack, the stack will only be torn down when ALL nodes have been torn down. This is by design to prevent premature cleanup.
