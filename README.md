# DART - Dynamic Assessment & Regression Toolkit

DART is a testing framework built to simplify the creation of complex, repeatable test scenarios across a variety of environments. Whether you're validating a single service or coordinating distributed systems, DART empowers you to automate environment setup, execution, and cleanup with minimal effort. Moreover, it integrates effortlessly into existing projects, enabling developers to include test definitions directly within their repositories so that upon cloning, they can immediately verify that all components are functioning as intended.

---

## Table of Contents

1. [Overview](#overview)  
2. [Key Features](#key-features)  
3. [Installation](#installation)  
4. [Usage](#usage)  
   - [Command Line Reference](#command-line-reference)  
   - [Exit Codes](#exit-codes)  
5. [Example Test Execution](#example-test-execution)  
6. [Example Test Definition](#example-test-definition)  
7. [License](#license)  

---

> **Notice:** This project is in an early development phase and may not yet be fully stable or feature complete. As it evolves, you may encounter significant changes to the API, behavior, and overall functionality.

## Overview

DART addresses the challenges of distributed systems testing by structuring workflows into **nodes**, **setup steps**, **tests**, and **teardown steps**. It supports various node types—from local processes and SSH remotes to Docker/LXD containers and virtual machines—while automating the configuration and testing processes. Its declarative YAML configuration allows you to embed test definitions directly within your project, so when you clone a repository, you can instantly run the tests to verify that your local environment is configured correctly.

---

## Key Features

- **Multiple Node Types**  
  Operates with localhost, remote SSH systems, containers (Docker/LXD), and virtual machines.

- **Automated Environment Preparation**  
  Provisions and configures nodes automatically, enabling on-demand creation of containers and virtual machines.

- **Declarative YAML Configuration**  
  Define your test suites in clear, maintainable YAML files that cover node configuration, setup, execution, and teardown.

- **Seamless Integration**  
  Easily embed test definitions within your existing projects so that a simple clone can yield a fully testable environment.

- **Setup and Teardown Hooks**  
  Run pre- and post-test operations to maintain a predictable and stable testing state.

- **Human-Readable Output**  
  Provides clear, color-coded test feedback, making it easy to see results at a glance.

- **DevOps Friendly**  
  Returns an exit code that reflects the outcome of the tests, integrating smoothly with CI/CD pipelines.

---

## Installation

Installation methods vary depending on your environment. Generally, you can:

- Clone the DART repository.
- Build from source or install via any officially supported package distribution.

*(Please refer to the official documentation for detailed installation instructions.)*

---

## Usage

### Command Line Reference

```vbnet
Usage: dart [OPTIONS] [ARGUMENTS]

Version: dev
Date: dev
Codebase: dev (dev)

Description: DART is a distributed systems testing framework
  designed to make it easy to perform automation and
  integration testing on a wide variety of distributed
  systems.

Options:
  Default: Default Options
    -c        --config          config.yaml  The path to the configuration file
    -v        --verbose         false        Enable verbose output
    -p        --pause-on-error  false        Pause on error
    -s        --stop-on-error   false        Stop on error
    -setup    --setup-only      false        Only run the setup steps
    -teardown --teardown-only   false        Only run the teardown steps
```

### Exit Codes

- **0**: All tests passed successfully.
- **Non-zero**: One or more tests failed or an unexpected error occurred.

These exit codes allow DART to integrate with automated DevOps workflows, ensuring that issues are immediately flagged during continuous integration and deployment processes.

---

## Example Test Execution

Below is a simplified example of how DART logs its operations during a test run. The actual output includes color coding and more detailed formatting for clarity:

```sql
[+] Running test setup
  running setup on localhost ......... done 
  running setup on locker-test ....... done 
  ensure sshpass is installed ........ done 
  ensure dns is working .............. done 
  install locker ..................... done 
  create user bob .................... done 
  create user jim .................... done 
  create user tom .................... done 
  ensure password login is allowed ... done 
  restart ssh ........................ done 

[+] Running tests
  00001: verify locker is installed .................. passed
  00002: test ........................................ passed
  00003: ssh to locker-test as bob ................... passed
  00004: ssh to locker-test as jim ................... passed
  00005: lock system as jim .......................... passed
  00006: ssh to locker-test as disallowed user bob ... passed
  00007: ssh to locker-test as allowed user tom ...... passed
  00008: unlock system as jim ........................ passed
  00009: verify bob can again access the system ...... passed

[+] Running test teardown
  running teardown on localhost ...... done 
  running teardown on locker-test .... done 

[+] Results
  Pass: 00009
  Fail: 00000
```

---

## Example Test Definition

The YAML configuration below demonstrates how to define nodes, setup steps, tests, and teardown operations. This example provisions and tests a tool called `locker` in an LXD container:

```yaml
---
suite: Locker End-to-End Tests
nodes:
  - name: localhost
    type: local
    options:
      shell: /bin/bash
  - name: locker-test
    type: lxd
    options:
      image: ubuntu:24.04
      type: container

setup:
  - name: ensure sshpass is installed
    node: localhost
    step:
      type: apt
      options:
        packages:
          - sshpass

  - name: ensure dns is working
    node: locker-test
    step:
      type: execute
      options:
        command: 'until nslookup github.com &>/dev/null; do sleep 1; done'

  - name: install locker
    node: locker-test
    step:
      type: execute
      options:
        command: "bash -o pipefail -c 'curl -fSL https://bgrewell.github.io/locker/install.sh | bash'"

  - name: create user bob
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash bob && echo 'bob:password123' | chpasswd"

  - name: create user jim
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash jim && echo 'jim:password123' | chpasswd"

  - name: create user tom
    node: locker-test
    step:
      type: execute
      options:
        command: "useradd -m -s /bin/bash tom && echo 'tom:password123' | chpasswd"

  - name: ensure password login is allowed
    node: locker-test
    step:
      type: execute
      options:
        command: "rm /etc/ssh/sshd_config.d/60-cloudimg-settings.conf"

  - name: restart ssh
    node: locker-test
    step:
      type: execute
      options:
        command: "systemctl restart ssh"

tests:
  - name: verify locker is installed
    node: locker-test
    type: execute
    options:
      command: "locker -h"
      evaluate:
        exit_code: 0

  - name: test
    node: localhost
    type: execute
    options:
      command: "whoami"
      evaluate:
        match: "ben"

  - name: ssh to locker-test as bob
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') whoami"
      evaluate:
        match: "bob"
        exit_code: 0

  - name: ssh to locker-test as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') whoami"
      evaluate:
        match: "jim"
        exit_code: 0

  - name: lock system as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') locker -r test -u tom lock"
      evaluate:
        contains: "Lock acquired"
        exit_code: 0

  - name: ssh to locker-test as disallowed user bob
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        exit_code: 255

  - name: ssh to locker-test as allowed user tom
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no tom@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        match: test
        exit_code: 0

  - name: unlock system as jim
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no jim@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') unlock"
      evaluate:
        contains: "Lock released"
        exit_code: 0

  - name: verify bob can again access the system
    node: localhost
    type: execute
    options:
      command: "sshpass -p 'password123' ssh -tt -o StrictHostKeyChecking=no -o PasswordAuthentication=yes -o PubkeyAuthentication=no bob@$(lxc list --project default locker-test --format csv -c4 | awk '{print $1}') echo test"
      evaluate:
        match: test
        exit_code: 0
```

---

## License

This project is distributed under an open-source or commercial license, as specified in the repository’s [LICENSE](LICENSE) file.

---

*Thank you for exploring DART! Your contributions and feedback are welcome as we strive to make testing in distributed environments as seamless as possible.*

---

Old todo list that needs to be migrated and cleaned up

## Task Types

- [ ] APT Package Management
- [ ] SNAP Package Management
- [ ] git clone
- [ ] command execution

## Next Tasks
- [ ] Flags
  - [ ] Verbose
  - [ ] Pause on fail
  - [ ] Stop on fail
  - [ ] Setup only
  - [ ] Teardown only
  - [ ] Skip teardown
  - [ ] Skip setup
- [ ] Support multiple nodes for tests
- [ ] More details on setup/docker steps when verbose is enabled
  
[x] - Summary of the test results
- Verbose output
- Failed test details
- General cleanup of formatter and controller
- Ability to load a series of tests from recursive folders of yaml files
- Ability to do something like monitor cpu usage over the test then evaluate at the end of the test what the avg cpu usage was
## Requirements

1. Execute test command/script and get results
2. Run results against a test function
3. Setup command(s) to prepare the system(s) for the test
4. Teardown command(s) to clean up the system(s) after the test
5. Load all tests from a directory
6. AI test evaluator


### Types of tests

- [x] Command Execution
- [ ] File Read
- [ ] File Write
- [ ] File Exist
- [ ] TCP Socket
- [ ] UDP Socket
- [ ] ICMP Socket
- [ ] Unix Socket
- [ ] HTTP Request
- [ ] HTTPS Request
- [ ] gRPC Request
- [ ] DNS Request
- [ ] (Plugins)
