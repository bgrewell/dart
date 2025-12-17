# File Operations Example

This example demonstrates the `file_create`, `file_edit`, and `file_delete` setup step types for managing files during test setup and teardown phases.

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

## Running the Example

```bash
dart run examples/file-operations/file-operations.yaml
```
