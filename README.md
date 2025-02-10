# DART

Dynamic Assessment & Regression Toolkit

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
