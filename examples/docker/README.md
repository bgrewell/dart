# Docker Examples

This directory contains examples for using Docker containers as test nodes in DART.

## Prerequisites

1. **Install Docker**: Follow the [Docker installation guide](https://docs.docker.com/engine/install/)
2. **User permissions**: Add your user to the `docker` group: `sudo usermod -aG docker $USER`
3. **For remote Docker**: Configure the remote Docker daemon or have SSH access to the remote host

## Examples

### docker.yaml - Local Docker Example

Demonstrates Docker containers as test nodes on the local machine:
- Basic container configuration
- Docker network setup
- Multi-container tests
- Custom images

```yaml
nodes:
  - name: my-container
    type: docker
    options:
      image: ubuntu:latest
      exec_opts:
        shell: /bin/bash
      networks:
        - name: test-net
          subnet: "172.20.0.0/16"
          ip: "172.20.0.2"
```

### docker-remote.yaml - Remote Docker Host Example

Demonstrates connecting to remote Docker hosts using environment variables:
- Remote Docker daemon via TCP with TLS
- Remote Docker via SSH tunnel
- Security best practices
- Mixed local/remote deployments

**Using TCP with TLS (Recommended for production):**

1. Configure the remote Docker daemon to accept TLS connections (see example file for details)

2. Set environment variables before running DART:
   ```bash
   export DOCKER_HOST=tcp://10.0.0.1:2376
   export DOCKER_TLS_VERIFY=1
   export DOCKER_CERT_PATH=/path/to/certs
   dart -c docker-remote.yaml
   ```

**Using SSH (Simple and secure):**

1. Ensure SSH access to the remote host:
   ```bash
   ssh user@remote-host docker ps
   ```

2. Set the DOCKER_HOST environment variable:
   ```bash
   export DOCKER_HOST=ssh://user@remote-host
   dart -c docker-remote.yaml
   ```

## Configuration Options

### Node Options

| Option | Description | Default |
|--------|-------------|---------|
| `image` | Docker image to use (e.g., `ubuntu:latest`, `nginx:alpine`) | Required |
| `exec_opts` | Execution options (e.g., `shell: /bin/bash`) | - |
| `networks` | Network configurations | - |

### Docker Configuration Section

Optional section to define Docker resources managed by the test suite:

```yaml
docker:
  networks:
    - name: test-net
      subnet: 172.20.0.0/16
      gateway: 172.20.0.1
  
  images:
    - name: custom-image
      tag: latest
      dockerfile: ./dockerfiles/Dockerfile.custom
```

### Remote Connection via Environment Variables

Docker remote connections are configured using standard Docker environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `DOCKER_HOST` | URL to the Docker server | `tcp://10.0.0.1:2376` or `ssh://user@host` |
| `DOCKER_TLS_VERIFY` | Enable TLS verification | `1` |
| `DOCKER_CERT_PATH` | Path to TLS certificates directory | `/path/to/certs` |
| `DOCKER_API_VERSION` | Docker API version (optional) | `1.41` |

**TLS Certificate Files** (in `DOCKER_CERT_PATH` directory):
- `ca.pem` - Certificate Authority certificate
- `cert.pem` - Client certificate
- `key.pem` - Client private key

## Common Docker Images

- `ubuntu:latest` - Latest Ubuntu LTS
- `ubuntu:24.04` - Ubuntu 24.04 LTS
- `debian:12` - Debian 12
- `alpine:latest` - Alpine Linux (minimal)
- `nginx:alpine` - Nginx web server
- `postgres:16` - PostgreSQL database
- `redis:alpine` - Redis cache

Browse more images at [Docker Hub](https://hub.docker.com/).

## Troubleshooting

**"Permission denied" error**: Ensure your user is in the `docker` group and re-login.

**"Cannot connect to the Docker daemon"**: Check if Docker is running with `systemctl status docker`.

**Remote connection fails**: 
- Verify network connectivity to the remote host
- Check TLS certificates are valid and accessible
- For SSH connections, verify SSH access works independently
- Check firewall rules allow Docker daemon port (default: 2376 for TLS)

**Image not found**: Pull the image manually with `docker pull <image>` to verify it exists.

**Container fails to start**: Check Docker logs with `docker logs <container-name>`.

## Security Best Practices

1. **Always use TLS** when connecting to remote Docker daemons over TCP
2. **Protect certificates**: Keep TLS certificates secure with appropriate file permissions (0600)
3. **Use SSH tunneling** when possible for simpler security model
4. **Avoid exposing Docker daemon** directly to the internet
5. **Regular updates**: Keep Docker daemon and client versions up to date
6. **Use non-root users** inside containers when possible
7. **Limit resources**: Use resource constraints in production environments
