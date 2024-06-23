# DART

Docker Automated Regression Testing

## Next Tasks
[x] - Summary of the test results
- Verbose output
- Failed test details
- General cleanup of formatter and controller
- Ability to load a series of tests from recursive folders of yaml files

## Requirements

1. Execute test command/script and get results
2. Run results against a test function
3. Setup command(s) to prepare the system(s) for the test
4. Teardown command(s) to clean up the system(s) after the test
5. Load all tests from a directory
6. AI test evaluator


### Types of tests

- Command Execution
- File Read
- File Write
- File Exist
- TCP Socket
- UDP Socket
- ICMP Socket
- Unix Socket
- HTTP Request
- HTTPS Request
- gRPC Request
- DNS Request
- (Plugins)