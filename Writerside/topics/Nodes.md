# Nodes in DART

Nodes define where your tests execute, whether on a **local machine, remote SSH server, container, or virtual machine**.

## Supported Node Types
DART supports the following types of nodes:
- **Local (`local`)** – Runs commands directly on the host machine.
- **SSH (`ssh`)** – Connects to remote machines via SSH.
- **Docker (`docker`)** – Runs tests inside Docker containers.
- **LXD (`lxd`)** – Runs tests in LXD containers.
- **LXD VM (`lxd-vm`)** – Runs tests in LXD virtual machines.

### Example Node Configuration
```yaml
nodes:
  - name: local
    type: local
    options:
      shell: /bin/bash

  - name: remote-server
    type: ssh
    options:
      host: example.com
      user: testuser
      key: ~/.ssh/id_rsa
```

---

## Booting a VM From an ISO

LXD nodes are usually created from an image. To test an installer instead, create an empty
virtual machine and attach the ISO as a boot device:

```yaml
nodes:
  - name: iso-vm
    type: lxd
    options:
      instance_type: virtual-machine

      # Create the VM with no image so it boots from its devices
      empty: true

      # Instance configuration keys, applied at creation
      config:
        security.secureboot: "false"

      devices:
        iso:
          type: disk
          source: work/output/example-0.1.0-amd64.iso
          # Rank the ISO above the root disk so the installer boots first
          boot.priority: 10

      # The VM is unreachable until the install finishes and it reboots from disk
      boot_wait:
        timeout: 1800          # Maximum seconds to wait (default 300)
        interval: 15           # Seconds between checks (default 2)
        initial_delay: 60      # Seconds before the first check (default 0)
        ready_command: cat /etc/hostname
```

`boot_wait` replaces the default readiness check. DART polls `ready_command` through the
node's shell until it exits zero or the timeout expires, so the tests that follow run against
the installed system rather than the installer. Relative `source` paths on disk devices are
resolved to absolute paths for local nodes; on remote nodes the source is a path on the remote
server and is passed through unchanged.

---

## Handling Sudo Privileges

Some test steps require **elevated privileges** (`sudo`). DART allows **four methods** to provide sudo credentials securely.

### 1️⃣ Using an Environment Variable (Recommended)
Set the password in an environment variable:
```sh
export SUDO_PASS="your-sudo-password"
```
Then reference it in your node configuration:
```yaml
nodes:
  - name: remote-server
    type: ssh
    options:
      host: example.com
      user: testuser
      sudo:
        env_var: "SUDO_PASS"
```
✅ **Safer than storing passwords in YAML.**  
✅ **Easy to rotate credentials dynamically.**

---

### 2️⃣ Using HashiCorp Vault (Future Feature)
For enterprise security, store the sudo password in **Vault**:
```yaml
nodes:
  - name: secure-server
    type: ssh
    options:
      host: secure.example.com
      user: admin
      sudo:
        vault_secret: "secret/data/sudo/password"
```
✅ **No plaintext passwords**  
✅ **Automated credential rotation**

*(Vault support is planned for a future release.)*

---

### 3️⃣ Using a Plaintext Password (⚠️ Not Recommended)
```yaml
nodes:
  - name: test-node
    type: local
    options:
      sudo:
        password: "my-sudo-password"
```
⚠️ **Avoid plaintext passwords** – They can be leaked in logs, backups, or version control.

---

### 4️⃣ Configuring Passwordless Sudo (Best Practice)
Modify `/etc/sudoers` to allow specific commands **without a password**:
```sh
sudo visudo
```
Add a rule:
```
testuser ALL=(ALL) NOPASSWD: /your/command
```
✅ **Most secure** for automation  
✅ **No password needed** in YAML or environment variables

---

### How DART Uses Sudo
When a test step requires `sudo`, DART checks for credentials in this order:
1. **Plaintext Password** (if set)
2. **Environment Variable** (if specified)
3. **HashiCorp Vault** (future feature)
4. **Passwordless Sudo** (if configured)

If no valid sudo credentials are available, the test **fails with an error**.

---
