# DART Documentation

> **Dynamic Assessment & Regression Toolkit (DART)**  
> A powerful framework for automating and validating complex test scenarios across various environments.

---

## What is DART?

DART is a **testing framework** designed to simplify the creation of **complex, repeatable test scenarios** across distributed environments. Whether you need to validate a single service or orchestrate tests across multiple systems, DART provides a **structured and automated** approach to environment setup, test execution, and teardown.

With **declarative YAML-based configuration**, DART makes it easy to integrate test definitions directly into your repositories, ensuring that **your entire system can be validated from the moment it's cloned**.

---

## Why Use DART?

- **Multi-Node Testing** – Run tests on **local, SSH, Docker, LXD, and virtual machines**.
- **Automated Environment Setup** – Provision test environments dynamically.
- **Declarative YAML Configuration** – Define tests clearly and maintainably.
- **Seamless Integration** – Embed tests directly in projects for **immediate validation**.
- **Setup and Teardown Hooks** – Keep test states clean and predictable.
- **CI/CD Friendly** – Exit codes enable **automated validation** in pipelines.

---

## How It Works

DART structures test execution using four key components:

1. **Nodes** – Define where tests run (local, remote, containerized, etc.).
2. **Setup Tasks** – Prepare the test environment (e.g., installing dependencies).
3. **Tests** – Validate system behavior with pass/fail conditions.
4. **Teardown Tasks** – Clean up resources to restore system state.

```yaml
suite: Example Test Suite
nodes:
  - name: local
    type: local
    options:
      shell: /bin/bash

setup:
  - name: Install Dependencies
    node: local
    step:
      type: apt
      options:
        packages:
          - curl
          - jq

tests:
  - name: Check API Response
    node: local
    step:
      type: execute
      options:
        command: "curl -s http://example.com/api | jq '.status'"
        evaluate:
          match: "success"

teardown:
  - name: Remove Temporary Files
    node: local
    step:
      type: execute
      options:
        command: "rm -rf /tmp/testdata"
```

---

## Getting Started

To begin using DART:

1. **[Install DART](Installation.md)** – Follow the setup guide.
2. **[Define Your First Test](Basic-Usage.md)** – Create a simple test.
3. **[Run Your Tests](Quick-Start.md)** – Execute and review results.

For a deep dive into DART’s features, explore the **[Core Concepts](Nodes.md)** section.

---

## Where to Go Next?

📖 **[Getting Started →](Quick-Start.md)**  
🛠️ **[Writing Tests →](Test-Structure.md)**  
📂 **[Configuration Reference →](Configuration.md)**  
🚀 **[Running DART in CI/CD →](CI-CD.md)**  
❓ **[FAQ & Troubleshooting →](Troubleshooting.md)**

---