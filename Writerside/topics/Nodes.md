# Nodes in DART

Nodes define where your tests execute, whether on a **local machine, remote SSH server, container, or virtual machine**.

## Supported Node Types
DART supports the following types of nodes:
- **Local (`local`)** – Runs commands directly on the host machine.
- **SSH (`ssh`)** – Connects to remote machines via SSH.
- **Docker (`docker`)** – Runs tests inside Docker containers.
- **LXD (`lxd`)** – Runs tests in LXD containers.

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
