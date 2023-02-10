# Testbeds
Testbeds must be defined in `exec/testbeds.yaml`. A testbed can be a HW testbed or sim. Example:
```yaml
testbeds:
  ...
  # Example HW testbed
  - id: 8808_FOX2506P2QT
    owner: arvbaska
    testbed: topologies/cisco/hw/8808_FOX2506P2QT/testbed
    binding: topologies/cisco/hw/8808_FOX2506P2QT/binding
    baseconf: topologies/cisco/hw/8808_FOX2506P2QT/baseconfig.proto
  # Example SIM testbed
  - id: 8201-1DUT-1ATE
    sim: true
    topology: topologies/cisco/pyvxr/8201-1DUT-1ATE/topo.yaml
    baseconf: topologies/cisco/pyvxr/8201-1DUT-1ATE/basefp.conf
  ...
```
**Note**: all paths must be replative to featureprofiles root.

## Per-test configuration
You can also specify specific testbed, binding, and/or baseconf to use for specific test cases. Example:
```yaml
testbeds:
  ...
  # Example HW testbed
  - id: 8808_FOX2506P2QT
    owner: arvbaska
    testbed: topologies/cisco/hw/8808_FOX2506P2QT/testbed
    binding: topologies/cisco/hw/8808_FOX2506P2QT/binding
    baseconf: topologies/cisco/hw/8808_FOX2506P2QT/baseconfig.proto
    overrides:
      P4RT-1.1:
        binding: topologies/cisco/hw/8808_FOX2506P2QT/binding_p4rt_1_1
  ...

```

# Run Lists
Run lists must be defined in `exec/tests` and have the following format:
```yaml
name: <unique_name>
owner: <cisco_email>
testbeds:                   # testbed will be picked at random from this list
    - testbed1_id           # tests will run in parallel on different testbeds
    - testbed2_id
    ...
tests:
    - name: <unique name>
      path: <test_path>     # relative to the root of featureprofiles
      args:
        - 'test arg 1'      # all args are concatenated as-is and passed to the test
        - 'test arg 2'
        ...
```

### Test execution
By default, tests are executed from the main branch of the public featureprofiles repository. This can be controlled by adding the following attributes per-test or at the testsuite level. Anything defined at the testuite level is considered default. A test that define an attribute overrides the one defined at the testsuite level.

| **Attribute** | **Description**                                                                                                                                                     | **Example**                                          |   |   |
|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------|---|---|
| internal      | When set to true the internal fp repository will be used instead. Default is false (i.e.,  public repo will be used).                                               | internal: true                                       |   |   |
| branch        | Branch from which the test should be executed. The branch must exist in the internal or public repository (depending on the value of internal). Defaults to 'main'. | branch: 'mybranch'                                   |   |   |
| revision      | Revision from which the test should be executed. Overrides 'branch'. Not set by default.                                                                            | revision: 'ca14b8f0245979574b5f34abbc9b9d4c525cc31e' |   |   |
| pr            | PR number from which the test should be executed. Overrides 'branch' and 'revision'. Not set by default.                                                            | pr: 847                                              |   |   |

### Testbed controls
You can control which test executes on which testbed by overriding the testsuite `testbeds` attribute in the test defintiion and including/excluding specific testbeds.

| **Attribute**    | **Description**                                                       | **Example**                                             |
|------------------|-----------------------------------------------------------------------|---------------------------------------------------------|
| testbeds         | Overrides the testsuite testbeds attribute                            | testbeds:     - 'my testbed id1'     - 'my testbed id2' |
| testbeds_include | Adds additional testbeds to the testsuite  testbeds attribute         | testbeds_include:     - 'additional testbed id'         |
| testbeds_exclude | Removes a testbed from the testsuite testbeds attribute for this test | testbeds_exclude:     - 'my testbed id1'                |


### Other attributes
| **Attribute** | **Description**                                                                                               | **Example**                                     |
|---------------|---------------------------------------------------------------------------------------------------------------|-------------------------------------------------|
| priority      | Test priority ( > 0) in increasing order. Tests with higher priorities are executed first.                    | priority: 1                                     |
| timeout       | Test timeout in seconds. Defaults to 1800s.                                                                   | testbeds_include:     - 'additional testbed id' |
| skip          | The test is skipped when set to true. Defaults to false.                                                      | skip: true                                      |
| mustpass      | Indicates that this test is known to pass before and a failure is considered a regression. Defaults to false. | mustpass: true                                  |

# FireX testsuite generator
The FireX testsuite generator takes the above yaml definition and generate a FireX testsuite for each test. Example:
```
go run ./exec/firex/v2/testsuite_generator.go -files 'runlist_1.yaml,runlist_2.yaml' > firex_testsuite.yaml
```

For each test, a FireX testsuite will be generated that looks like:
```yaml
gNOI-5.1 (Ping Test):
    framework: b4_fp
    owners:
        - arvbaska@cisco.com
    testbeds:
        - 8808-1DUT-1ATE
    supported_platforms:
        - "8000"
    script_paths:
        - (25) gNOI-5.1 (Ping Test) (MP):
            test_name: gNOI-5.1
            test_path: feature/gnoi/system/tests/ping_test
            test_args: 
              -deviation_subinterface_packet_counters_missing="true" -deviation_traceroute_l4_protocol_udp="true"
            internal_test: false
            test_timeout: 0
    smart_sanity_exclude: True
```

### Flags
| **Flag**            | **Description**                                                            | **Example**                               |
|---------------------|----------------------------------------------------------------------------|-------------------------------------------|
| -test_names         | Generate only for these tests.                                             | -test_names 'gNOI-5.1,TE-3.1'             |
| -exclude_test_names | Exclude these tests.                                                       | -exclude_test_names 'gNOI-4.1,TE11.1'     |
| -testbeds           | Override the run list testbeds attribute.                                  | -testbeds 'my_testbed1_id,my_testbed2_id' |
| -sort               | Sort tests by priority. Set by default.                                    | -sort false                               |
| -randomize          | Sort tests in random order.                                                | -randomize                                |
| -must_pass_only     | Generate only tests with mustpass = true.                                  | -must_pass_only                           |
| -use_short_names    | Use test name as is for FireX script name.                                 | -use_short_names                          |
| -out_dir            | Write each test in a seperate FireX testsuite file in the given directory. | -our_dir 'my_firex_suite_directory'       |
| -env                | Env variables added to FireX testsuite.                                    | -env 'GH_TOKEN=x,GH_HOST=github.com'      |
| -plugins            | Extra FireX plugins to add to the testsuite.                               | -plugins 'vxr.py,cflow.py'                |

# Using FireX
Since our runner plugin is not in the FireX store, you need to copy it somewhere accessible from an ADS machine. For example:
```
cp exec/firex/plugins/v2/runner.py /auto/tftpboot-ottawa/kjahed/firex/v2/runner.py
```
Then, you can submit a FireX run (on HW) using the command:
```
/auto/firex/bin/firex submit --chain RunTests --plugins /auto/tftpboot-ottawa/kjahed/firex/v2/runner.py --testsuite firex_testsuite.yaml --images dummy.iso
```
Note that the `--images` argument is mandatory although it is not actually used. That's it, for HW runs
we assume that the image is already loaded.

If you're running on sim (i.e., `testbeds` in the run list points to a sim testbed), then you can skip the `--images` flag and FireX will build the latest image and load it on to a sim. You can also specify the path to an iso and FireX will load that instead.

You can also override any of the properties in the generated `firex_testsuite.yaml`. For example, to force all tests to run from an internal branch called 'mybranch', you can do:

```
/auto/firex/bin/firex submit --chain RunTests --plugins /auto/tftpboot-ottawa/kjahed/firex/v2/runner.py --testsuite firex_testsuite.yaml --test_branch 'mybranch' --internal_test True
```
