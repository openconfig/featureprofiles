### **1. General Coding & Contribution Guidelines**

**Source:** `CONTRIBUTING.md`

*   **License Requirement:** All code must be Apache 2.0 licensed, and authors
    must sign the Google CLA.
*   **Go Code Style:**

    *   Follow
        [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments),
        [Effective Go](https://go.dev/doc/effective_go), and
        [Google Go Style Guide](https://google.github.io/styleguide/go/) for
        writing readable Go code with a consistent look and feel.

    *   Tests should follow
        [Testing on the Toilet](https://testing.googleblog.com/) best practices.

    *   Use
        [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests),
        but **do not** run test cases in parallel.

*   **Directory Structure:**

    *   Directory names must **not** contain hyphens (`-`).
    *   Tests must be nested under `tests/` or `otg_tests/` directories.
    *   Organization format:
        `feature/<featurename>/[<sub-feature>/]<tests|otg_tests|kne_tests>/<test_name>/<test_name>.go`.
    *   Adding sub-feature folders is preferred over creating new feature
        folders.  Before adding a new feature folder, scan the existing
        folders for a suitable match
    *   If a new top level feature is introduced, `.github/CODEOWNERS`
        must also be updated to specify who the owners of the feature
        will be.

*   **Code Should Follow The Test README:**

    *   The test `README.md` should be structured following the
        [test plan template]([url]\(https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md\)).

    *   Each step in the test plan README should correspond to a comment or
        `t.Log`in the code. Steps not covered by code should have a TODO comment
        in the test code.

    *   In the PR, please mention any corrections made to the test README for
        errors that were discovered when implementing the code.

*   **File Types:**

    *   Source code, text-formatted protos, and device configs must **not** be
        executable.
    *   Only scripts (Shell, Python, Perl) may be executable.

*   **Test Code Structure:**

    *   Test code must follow the steps documented in the test `README.md`.
    *   Environment setup code should be placed in a function named
        setupEnvironment and called from TestMain
    *   Use `t.Run` for subtests so output clearly reflects passed/failed steps.
    *   **Eliminating Polling Loops and Static Sleeps (CRITICAL / HIGH PRIORITY):**
        *   **Anti-pattern:** Relying on static `time.Sleep` calls, or building custom `for` loops that poll telemetry state with `time.Sleep(pollInterval)`.
        *   **Correction:** The use of `gnmi.Watch` with `.Await(t)` is mandatory to detect desired states in real-time. Custom polling loops are banned.
        *   **BAD (Do not do this):**
            ```go
            for {
                val := gnmi.Get(t, dut, path)
                if val == expected { break }
                time.Sleep(2 * time.Second)
            }
            ```
        *   **GOOD (Do this instead):**
            ```go
            gnmi.Watch(t, dut, path, timeout, func(val *ygnmi.Value[Type]) bool {
                v, present := val.Val()
                return present && v == expected
            }).Await(t)
            ```
    *   **Querying Counters:** Always use a `gnmi.Watch` loop to wait for a specific counter value to be reached before querying it.
    *   All variables used as denominators in arithmetic operations (especially dynamic metric counters like `txPackets`) must be explicitly checked against zero before division to prevent runtime panics.
    *   **OTG Start Protocols:** Prior to invoking OTG start protocols, explicitly call `WaitForARP` function to maintain test stability.
      
        *   **OTG traffic neighbor resolution:** For tests that configure and
                start OTG (traffic generator) protocols and traffic, ensure neighbor
                resolution (ARP for IPv4 or ND for IPv6) has completed before
                starting traffic. Specifically:

                - After `StartProtocols(t)` and before `StartTraffic(t)`, call the
                    appropriate helper: `otgutils.WaitForARP(t, otg, config, "IPv4")`
                    or `otgutils.WaitForARP(t, otg, config, "IPv6")` depending on the
                    address family used by the flows.
                - If a helper function calls `StartTraffic()`, the caller must ensure
                    the matching `WaitForARP(...)` has already completed; do not rely on
                    `StartTraffic()` to implicitly resolve neighbors.
                - For dual-stack tests that send both IPv4 and IPv6 traffic, wait for
                    both families before starting traffic.

                This prevents initial packets from being dropped due to unresolved
                neighbor entries and makes traffic validation deterministic.

*   **Enums:**

    *   Do not use numerical enum values (e.g., `6`). Use the ygot-generated
        constants (e.g., `telemetry.PacketMatchTypes_IP_PROTOCOL_IP_TCP`).

*   **Network Assignments:**

    *   **IPv4:** Use RFC 5737 blocks (`192.0.2.0/24`, `198.51.100.0/24`,
        `203.0.113.0/24`) or `100.64.0.0/10`, `198.18.0.0/15`.
    *   **IPv6:** Use RFC 3849 (`2001:DB8::/32`), sub-divided for control/data
        planes.
    *   **ASNs:** Use RFC 5398 (`64496-64511` or `65536-65551`).
    *   **Do not use:** `1.1.1.1`, `8.8.8.8`, or common local private ranges
        like `192.168.0.0/16`.

*   **Configuration & Cleanup**
    *   **Mandatory State Reversion:** Tests must always leave the system in the exact original state it was in prior to the test execution, regardless of whether the test passes or fails. This requirement applies to both standard gNMI `Set` configurations and raw/native CLI commands (e.g., using `helpers.GnmiCLIConfig`). The PR MUST include corresponding cleanup operations to revert any changes made during the test.
    *   **SSH and AAA State:** Pay special attention to AAA and SSH configurations. If a test modifies SSH authentication (e.g., `management ssh authentication protocol password`), the cleanup routine MUST explicitly negate that specific command (e.g., `management ssh \n no authentication protocol`) rather than relying on generic default commands that might wipe baseline lab configurations.
    *   **Use `t.Cleanup()`:** All cleanup operations, whether for gNMI configurations or raw CLI commands, must be registered using `t.Cleanup()` to guarantee they are executed even if the test fails or panics early.
    *   **README Cleanup Specification:** All test plan `README.md` files must explicitly include a cleanup/teardown step (or `### Cleanup` section) specifying that any state or configuration modified during the test (e.g., disabled interfaces, drained links, altered protocol states) is reverted and the DUT is restored to its baseline operational state upon test completion. Reviewers must flag `README.md` test plans that leave interfaces or protocols in a degraded/disabled state without an explicit cleanup step.
    
*   **Bug references:**

    * References to bugs may be made using deviations.  Accessible URL format should be used, such as https://issuetracker.google.com/xxxx format links in the internal/deviations/deviations.textproto file.
    * TODO items in a README may be documented.  It is preferred to reference to GitHub repositories or other publicly accessible URLs.  Bug tracking URLs may be used as a last resort.
    * Internal project tracking bugs should not be referenced in README or test code.

### **2. README Guidelines**
**Source:** `doc/test-requirements-template.md`

    *   The test `README.md` should be structured following the
        [test plan template]([url]\(https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md\)).

    *   READMEs should include the following verification patterns:
    1. Routing, control plane programming and resilience.  For example, tests should not only verify that control plane state can be established, but also that it survives changes such as links going down, adjacencies change, rapid addition and removal of entries.  
    2. Operations & State Changes should be defined in the README.  For example, setting Port and Linecard admin states Up/Down under high traffic load.  Admin down of the individual aggregate interface members.  
    3. High availability should be tested by performing active/standby failover between redundant controller cards if the author deems high availability is in scope.
    4. Minimal scale should be part of all tests.  This involves using two or more of the entities being tested.  (control plane adjacencies, interfaces, traffic flows, configuration values).
    5. Larger scale and performance tests should be dedicated, separate tests as they may take significant resources and time to execute.  Making them separate tests allows them to be scheduled appropriately in constrained test environments.  
    6.  Checking TTL, DSCP, ECN preservation across encapsulation and decapsulation.
    7.  Checking traffic load balancing performance for encapsulated traffic, especially IP-in-IP and MPLSoUDP.
    8. Validation of traffic changes needed for network maintenance.  Control plane changes should be introduced which validate shifting traffic off of links, aggregates using control plane changes.
    9. Telemetry must be validated to confirm counters are streamed at the expected intervals.  Doing a simple one time Get is not sufficient.  Tests using gRIBI should validate device returns complete and accurate routing entries during a gRIBI Get() response and through gNMI AFT reporting, matching the programmed state.
    10. Some tests may include coverage for software stability.  These should trigger and validate that the DUT reconciles its internal state without data plane traffic loss, restores connections, and remains stable during concurrent rotations without core dumps. 
    11. Tests with high scale should validate hardware resource management using the `/components/component/integrated-circuit/utilization/resources` model.
  
    *   Strictly scoped feature tests are permitted, but are not the only tests in scope for the featureprofiles repository.  Multidimensional tests may be created containing a group of features that interact.  For example, QoS, aggregate interfaces, ACLs and policy-forwarding.  Which features are chosen is at the discretion of the test author.  

        
### **3. Deviation Guidelines**

**Source:** `internal/deviations/README.md`

*   **When to use:** Use deviations to allow alternate OC paths or CLI commands
    to achieve the *same* operational intent. Do not use them to skip validation
    or change the intent of the test.
*   **Implementation Steps:**
    1.  Define the deviation in `proto/metadata.proto`.
    2.  Generate Go code using `make proto/metadata_go_proto/metadata.pb.go`.
    3.  Add an accessor function in `internal/deviations/deviations.go`. This
        must accept `*ondatra.DUTDevice`.
    5.  Add a comment to the accessor function containing a URL link to an
        issue tracker which tracks removal of the deviation.  The format should
        be `https://issuetracker.google.com/issues/xxxxx`.  If the issue is not
        tracked at Google, another URL could be used.
    7.  Enable the deviation in the test's `metadata.textproto` file.

*   **Usage in Tests:** Access deviations via `deviations.DeviationName(dut)`.


### **4. Configuration Plugins (`cfgplugins`) Guidelines**

**Source:** `internal/cfgplugins/README.md`

*   **Purpose:** Use `cfgplugins` to generate reusable configuration snippets
    for the DUT.
*   **Implementation:**
    *   Functions should align with the `/feature` folder structure.
    *   Use a struct to pass configuration parameters.
    *   **Function Signature:** Must be `(t *testing.T, dut *ondatra.DUTDevice,
        sb *gnmi.SetBatch, cfg MyConfigStruct) *gnmi.SetBatch`.
*   **Workflow:**
    *   The plugin appends config to the `*gnmi.SetBatch` object.
    *   The **calling test code** is responsible for executing `batch.Set(t,
        dut)` to apply the config.
*   **Deviations:** Deviations that affect configuration generation should be
    implemented *inside* the `cfgplugins` function, not in the test file.

### **5. Performance & Execution Optimizations**

To achieve high-velocity test execution and prevent pipeline bottlenecks, reviewers MUST flag the following performance anti-patterns and suggest the provided alternatives:

*   **Batching gNMI Operations (Avoid "N+1" Queries):**
    *   **Anti-pattern:** Using serial `gnmi.Get` calls inside a loop to query multiple ports or leaves. The "N+1" query pattern is a major contributor to setup and execution slowness.
    *   **Correction:** Use `gnmi.OCBatch()` to aggregate paths and perform a single RPC call. 
    *   *Example Correction:* 
        ```go
        batch := gnmi.OCBatch()
        for _, p := range ports {
            batch.AddPaths(gnmi.OC().Interface(p).OperStatus())
        }
        results := gnmi.Get(t, dut, batch.State())
        ```
*   **Sequential Validations:**
    *   **Anti-pattern:** Verifying hundreds of neighbors or telemetry counters sequentially in a loop, which leads to idle device time.
    *   **Correction:** Use `errgroup.Group` to issue gNMI telemetry requests concurrently. This collapses the total latency from `N×RTT` to `≈1×RTT`.
*   **Setup Redundancy:**
    *   **Anti-pattern:** Performing a full teardown and rebuild for every subtest, especially when significant overlap exists between tests.
    *   **Correction:** Group related functional checks using `t.Run` to reduce redundant environment setup cycles. Use a `SetupDUTOnce()` pattern to check existing configurations and skip expensive `gnmi.Replace` and reboot phases if the DUT is already in the desired state.
*   **Eliminating Static Sleeps (CRITICAL / HIGH PRIORITY):**
    *   **Anti-pattern:** Relying on static `time.Sleep` calls based on legacy assumptions. **This is a critical performance issue that must be avoided and flagged as high priority.**
    *   **Correction:** The use of `gnmi.Watch` is mandatory to detect desired states in real-time. Wait times should not be hardcoded, as static sleeps become a fixed tax on the pipeline.
