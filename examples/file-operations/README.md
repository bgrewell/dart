# File Operations Example

This example demonstrates the `file_create`, `file_write`, `file_edit`, `file_read`, `file_exists`, and
`file_delete` setup step types for managing files during test setup and teardown phases.

File operations act on the filesystem of the node named in the step, not on the machine running DART.
A step attached to an `ssh`, `docker`, `docker-compose`, or `lxd` node reads and writes files inside
that node. See [Node Support](#node-support) for the requirements each node type places on this.

## Step Types Demonstrated

### `file_create`

Creates a new file with specified content.

**Options:**
- `path` (required): Path to the file to create
- `contents`: Content to write to the file
- `overwrite`: If `true`, overwrites existing file; if `false` (default), fails if file exists
- `create_dir`: If `true`, creates parent directories if they don't exist
- `mode`: File permission mode (default: 0644)

```yaml
- name: create config file
  node: local
  step:
    type: file_create
    options:
      path: /tmp/config.txt
      contents: "key=value"
      create_dir: true
      overwrite: true
```

### `file_write`

Writes content to a file. Unlike `file_create` it does not create parent directories and does not take
a mode, so it is the simpler choice when the directory already exists.

**Options:**
- `path` (required): Path to the file to write
- `contents`: Content to write to the file
- `overwrite`: If `true`, overwrites an existing file; if `false` (default), fails if the file exists

```yaml
- name: write inventory
  node: local
  step:
    type: file_write
    options:
      path: /tmp/hosts.ini
      contents: |
        [core]
        node1
      overwrite: true
```

### `file_edit`

Modifies file contents using insert, replace, or remove operations.

**Options:**
- `path` (required): Path to the file to edit
- `operation` (required): One of `insert`, `replace`, or `remove`
- `match_type`: How to find content - `plain`, `regex`, or `line` (default: `plain`)
- `match`: Pattern to match (required unless `match_type` is `line`)
- `position`: For insert - `before` or `after` (default: `after`)
- `line_number`: For `match_type: line` - line number to insert at
- `content`: Content to insert or replacement text
- `use_captures`: For regex replace - enables capture group replacement (`$1`, `$2`, `${name}`)

#### Insert by line number
```yaml
- name: add line after line 5
  node: local
  step:
    type: file_edit
    options:
      path: /tmp/file.txt
      operation: insert
      match_type: line
      line_number: 5
      position: after
      content: "new line content"
```

#### Replace with plain text
```yaml
- name: replace text
  node: local
  step:
    type: file_edit
    options:
      path: /tmp/file.txt
      operation: replace
      match_type: plain
      match: "old_value"
      content: "new_value"
```

#### Replace with regex and capture groups
```yaml
- name: update version
  node: local
  step:
    type: file_edit
    options:
      path: /tmp/version.txt
      operation: replace
      match_type: regex
      match: "version=(\\d+)\\.(\\d+)\\.(\\d+)"
      content: "version=$1.$2.999"
      use_captures: true
```

#### Remove content
```yaml
- name: remove comments
  node: local
  step:
    type: file_edit
    options:
      path: /tmp/config.txt
      operation: remove
      match_type: regex
      match: "#.*\n"
```

### `file_read`

Reads a file and optionally checks its content. Useful as a setup-time assertion that a file landed
on the node with the content you expect.

**Options:**
- `path` (required): Path to the file to read
- `contains`: Substring that must appear in the file; the step fails if it is missing

```yaml
- name: verify inventory was written
  node: local
  step:
    type: file_read
    options:
      path: /tmp/hosts.ini
      contains: "[core]"
```

### `file_exists`

Checks that a file exists. The step fails if the path is missing.

**Options:**
- `path` (required): Path to check

```yaml
- name: confirm config is present
  node: local
  step:
    type: file_exists
    options:
      path: /tmp/config.txt
```

### `file_delete`

Deletes a file.

**Options:**
- `path` (required): Path to the file to delete
- `ignore_errors`: If `true`, doesn't fail when file doesn't exist (useful for cleanup)

```yaml
- name: cleanup temp file
  node: local
  step:
    type: file_delete
    options:
      path: /tmp/temp.txt
      ignore_errors: true
```

## Node Support

Every file step runs against the node it is attached to. How that happens differs per node type, and
each transport brings its own requirements:

| Node type | Transport | Requirements |
|-----------|-----------|--------------|
| `local` | `os` package | None |
| `ssh` | SFTP over the existing SSH connection | The remote host must run an SFTP subsystem (the default on most distributions) |
| `lxd` / `lxd-vm` | LXD instance file API | The instance must be running. Virtual machines additionally need the LXD guest agent |
| `docker` | tar streams over the Docker copy API, plus in-container commands | The image must provide `rm`, `mkdir`, and `chmod` for delete and directory creation |
| `docker-compose` | Same as `docker`, resolved to the service's container | Same as `docker` |

Things worth knowing before you hit them:

- **LXD virtual machines need the guest agent.** File operations use the instance file API, which is
  served by the agent inside the VM. A VM without it — including one still running an unattended
  install from an ISO, before `boot_wait` has completed — will fail these steps even though `execute`
  steps may work. Containers do not need the agent.
- **Minimal container images may not support delete or `create_dir`.** Removal and directory creation
  shell out to `rm`, `mkdir`, and `chmod` inside the container, so distroless and `scratch` images
  will fail those steps. Reads and writes use tar streams and work regardless.
- **The destination directory must already exist** for a plain write. Use `file_create` with
  `create_dir: true` when it may not.
- **The overwrite guard is a check, not a lock.** `file_create` and `file_write` test for an existing
  file and then write, so the guard is not atomic against something else creating the file in between.

## Running the Example

```bash
dart run examples/file-operations/file-operations.yaml
```
