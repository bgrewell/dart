# Basic Usage

### **Key Differences Between Setup/Teardown & Tests**

| **Aspect**         | **Setup/Teardown**                     | **Tests**                          |
|-------------------|--------------------------------|--------------------------------|
| **Purpose**      | Prepares or cleans up the environment | Validates behavior or correctness |
| **Execution Style** | Runs commands but does not evaluate their output | Runs commands **and** evaluates their output |
| **Validation**   | Considered "successful" if it completes without errors | Has explicit **pass/fail conditions** based on output |
| **Failure Impact** | Failing setup stops execution; failing teardown logs errors but doesn't block tests | Failure means the test fails (but may continue based on settings) |
| **Examples**     | Installing dependencies, configuring services, creating directories | Checking API response codes, verifying file contents, asserting socket communication |
