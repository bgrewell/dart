# Command Line Reference

Flags, exit codes, and CI integration for the `dart` command.

[← Back to the README](../README.md)

### Command Line Reference

```bash
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

### CI Integration

```bash
dart -c suite.yaml -r junit:results.xml,json:results.json   # test panels + tooling
dart -c suite.yaml -l run.log                               # clean transcript (no colors/spinners)
dart -c suite.yaml --check                                  # validate config + print plan, run nothing
```

JUnit output feeds GitHub/GitLab/Jenkins test panels (skips and failure
details included); JSON carries the same data plus durations for custom
tooling. `--check` validates node types, report specs, and the full option set of
every step and test against mock nodes — a pre-commit or CI lint that
touches no infrastructure (node connectivity is not exercised). The results summary shows total suite time. With `-i N`, each iteration
writes its own report (`results-1.xml`, `results-2.xml`, ...) so a passing
final iteration can't mask an earlier failure; reports are also written
when a run aborts early (teardown failure, stop-on-error), and `--log`
captures debug-streamed command output too.

### Exit Codes

- **0**: All tests passed successfully.
- **Non-zero**: One or more tests failed or an unexpected error occurred.

These exit codes allow DART to integrate with automated DevOps workflows, ensuring that issues are immediately flagged during continuous integration and deployment processes.

---

## Example Test Execution

Below is a simplified example of how DART logs its operations during a test run. The actual output includes color coding and more detailed formatting for clarity:

```bash
[+] Running test setup
  running setup ...................... done 
  running setup ...................... done 
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
  00002: ssh to locker-test as bob ................... passed
  00003: ssh to locker-test as jim ................... passed
  00004: lock system as jim .......................... passed
  00005: ssh to locker-test as disallowed user bob ... passed
  00006: ssh to locker-test as allowed user tom ...... passed
  00007: unlock system as jim ........................ passed
  00008: verify bob can again access the system ...... passed

[+] Running test teardown
  running teardown ................... done 
  running teardown ................... done 

[+] Results
  Pass: 00008
  Fail: 00000
```

---
