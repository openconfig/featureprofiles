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

*   **Code Should Follow The Test README:**

    *   The test `README.md` should be structured following the
        [test plan template]([url]\(https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md\)).

    *   Each step in the test plan procedure should correspond to a comment or
        `t.Log`in the code. Steps not covered by code should have a TODO comment
        in the test code.

    *   In the PR, please mention any corrections made to the test README for
        errors that were discovered when implementing the code.

*   **File Types:**

    *   Source code, text-formatted protos, and device configs must **not** be
        executable.
    *   Only scripts (Shell, Python, Perl) may be executable.

*   **Test Structure:**

    *   Test code must follow the steps documented in the test `README.md`.
    *   Environment setup code should be placed in a function named
        setupEnvironment and called from TestMain
    *   Use `t.Run` for subtests so output clearly reflects passed/failed steps.
    *   **Avoid `time.Sleep`**: Use `gnmi.Watch` with `.Await` for waiting on
        conditions.

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

### **2. Deviation Guidelines**

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


### **3. Configuration Plugins (`cfgplugins`) Guidelines**

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

