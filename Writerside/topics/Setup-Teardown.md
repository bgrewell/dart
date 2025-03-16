# Setup &amp; Teardown

### **MVP Step Types**

The following step types are included in the initial **MVP version** of DART:

| **Step Type**       | **Purpose** | **Description** |
|--------------------|------------|----------------|
| **Simulated**      | Structured delays for debugging. | Adds artificial wait times to simulate long-running processes or dependencies. |
| **Execute**        | Runs shell commands. | Executes a command on the specified node. Considered successful if the exit code is `0`. |
| **Package Manager** | Installs system packages. | Supports multiple package managers (`apt`, `dnf`, `yum`, `apk`, `brew`, etc.). |
| **File Write**     | Creates/modifies file contents. | Writes a specified string into a file, overwriting if necessary. |
| **File Exists**    | Checks if a file exists. | Validates that a file is present on the system without reading its contents. |
| **File Read**      | Reads and validates file contents. | Reads a file and verifies that it contains expected content. |
| **HTTP Request**   | Tests HTTP endpoints. | Sends an HTTP request and validates the response code, headers, or body. |
| **DNS Request**    | Validates domain resolution. | Ensures a hostname resolves to an expected IP address. |
| **Service Check**  | Ensures system services are running. | Verifies whether a systemd or other service is active on the target node. |

---

### **Future Step Types**
The following step types are planned for future versions of DART:

| **Step Type**       | **Purpose** | **Planned Feature** |
|--------------------|------------|----------------|
| **File Delete**    | Removes files or directories. | Deletes specified files or folders during setup or teardown. |
| **TCP/UDP Socket Check** | Verifies network connectivity. | Opens a socket connection to test communication between nodes. |
| **gRPC Request**   | Tests gRPC APIs. | Sends a gRPC request and validates the response. |
| **SNAP Package Management** | Installs software via Snap. | Adds support for installing and managing Snap packages. |

---

### **Why These Steps?**
The **MVP step types** were selected to cover:
- **Basic system automation** (file handling, service validation, package installation).
- **Network and API testing** (HTTP, DNS validation).
- **Command execution** (core test functionality).

For a complete list of step types and their configurations, see the [Step Reference](../reference/step-types.md).

